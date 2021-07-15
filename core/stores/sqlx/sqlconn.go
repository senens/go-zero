package sqlx

import (
	"database/sql"
	"errors"
	"github.com/tal-tech/go-zero/core/logx"
	"regexp"
	"strings"

	"github.com/tal-tech/go-zero/core/breaker"
)

// ErrNotFound is an alias of sql.ErrNoRows
var (
	ErrNotFound     = sql.ErrNoRows
	DefaultMatchSql = "SELECT"
)

type (
	// Session stands for raw connections or transaction sessions
	Session interface {
		Exec(query string, args ...interface{}) (sql.Result, error)
		Prepare(query string) (StmtSession, error)
		QueryRow(v interface{}, query string, args ...interface{}) error
		QueryRowPartial(v interface{}, query string, args ...interface{}) error
		QueryRows(v interface{}, query string, args ...interface{}) error
		QueryRowsPartial(v interface{}, query string, args ...interface{}) error
	}

	// SqlConn only stands for raw connections, so Transact method can be called.
	SqlConn interface {
		Session
		Transact(func(session Session) error) error
	}

	// SqlOption defines the method to customize a sql connection.
	SqlOption func(*commonSqlConn)

	// StmtSession interface represents a session that can be used to execute statements.
	StmtSession interface {
		Close() error
		Exec(args ...interface{}) (sql.Result, error)
		QueryRow(v interface{}, args ...interface{}) error
		QueryRowPartial(v interface{}, args ...interface{}) error
		QueryRows(v interface{}, args ...interface{}) error
		QueryRowsPartial(v interface{}, args ...interface{}) error
	}

	// thread-safe
	// Because CORBA doesn't support PREPARE, so we need to combine the
	// query arguments into one string and do underlying query without arguments
	commonSqlConn struct {
		driverName string
		datasource map[string]string
		cluster    bool
		beginTx    beginnable
		brk        breaker.Breaker
		accept     func(error) bool
	}

	sessionConn interface {
		Exec(query string, args ...interface{}) (sql.Result, error)
		Query(query string, args ...interface{}) (*sql.Rows, error)
	}

	statement struct {
		stmt *sql.Stmt
	}

	stmtConn interface {
		Exec(args ...interface{}) (sql.Result, error)
		Query(args ...interface{}) (*sql.Rows, error)
	}
)

// NewSqlConn returns a SqlConn with given driver name and datasource.
func NewSqlConn(driverName string, datasource map[string]string, cluster bool, opts ...SqlOption) SqlConn {
	conn := &commonSqlConn{
		driverName: driverName,
		datasource: datasource,
		cluster:    cluster,
		beginTx:    begin,
		brk:        breaker.NewBreaker(),
	}
	for _, opt := range opts {
		opt(conn)
	}

	return conn
}

func (db *commonSqlConn) DataSourceResp(q string, cluster bool, datasource map[string]string) (string, error) {
	if cluster {
		reg := regexp.MustCompile(`(?U)^.* `).FindAllString(strings.TrimSpace(q), -1)
		if strings.ToUpper(strings.TrimSpace(reg[0])) == DefaultMatchSql {
			if _, ok := datasource["slave"]; ok {
				return datasource["slave"], nil
			} else {
				return "", errors.New("dataSource config slave error")
			}
		}
	}

	if _, ok := datasource["master"]; ok { //only use master
		return datasource["master"], nil
	} else {
		return "", errors.New("dataSource config master error")
	}
}

func (db *commonSqlConn) Exec(q string, args ...interface{}) (result sql.Result, err error) {
	err = db.brk.DoWithAcceptable(func() error {
		datasource, err := db.DataSourceResp(q, db.cluster, db.datasource)
		logx.Infof("exec DataSourceResp data %v,%v,%v,%v", q, db.cluster, db.datasource, datasource)
		if err != nil {
			logInstanceError(datasource, err)
			return err
		}

		var conn *sql.DB
		conn, err = getSqlConn(db.driverName, datasource)
		if err != nil {
			logInstanceError(datasource, err)
			return err
		}

		result, err = exec(conn, q, args...)
		return err
	}, db.acceptable)

	return
}

func (db *commonSqlConn) Prepare(query string) (stmt StmtSession, err error) {
	err = db.brk.DoWithAcceptable(func() error {
		datasource, err := db.DataSourceResp(query, db.cluster, db.datasource)
		if err != nil {
			logInstanceError(datasource, err)
			return err
		}

		var conn *sql.DB
		conn, err = getSqlConn(db.driverName, datasource)
		if err != nil {
			logInstanceError(datasource, err)
			return err
		}

		st, err := conn.Prepare(query)
		if err != nil {
			return err
		}

		stmt = statement{
			stmt: st,
		}
		return nil
	}, db.acceptable)

	return
}

func (db *commonSqlConn) QueryRow(v interface{}, q string, args ...interface{}) error {
	return db.queryRows(func(rows *sql.Rows) error {
		return unmarshalRow(v, rows, true)
	}, q, args...)
}

func (db *commonSqlConn) QueryRowPartial(v interface{}, q string, args ...interface{}) error {
	return db.queryRows(func(rows *sql.Rows) error {
		return unmarshalRow(v, rows, false)
	}, q, args...)
}

func (db *commonSqlConn) QueryRows(v interface{}, q string, args ...interface{}) error {
	return db.queryRows(func(rows *sql.Rows) error {
		return unmarshalRows(v, rows, true)
	}, q, args...)
}

func (db *commonSqlConn) QueryRowsPartial(v interface{}, q string, args ...interface{}) error {
	return db.queryRows(func(rows *sql.Rows) error {
		return unmarshalRows(v, rows, false)
	}, q, args...)
}

func (db *commonSqlConn) Transact(fn func(Session) error) error {
	return db.brk.DoWithAcceptable(func() error {
		return transact(db, db.beginTx, fn)
	}, db.acceptable)
}

func (db *commonSqlConn) acceptable(err error) bool {
	ok := err == nil || err == sql.ErrNoRows || err == sql.ErrTxDone
	if db.accept == nil {
		return ok
	}

	return ok || db.accept(err)
}

func (db *commonSqlConn) queryRows(scanner func(*sql.Rows) error, q string, args ...interface{}) error {
	var qerr error
	return db.brk.DoWithAcceptable(func() error {
		datasource, err := db.DataSourceResp(q, db.cluster, db.datasource)
		logx.Infof("exec DataSourceResp data %v,%v,%v,%v", q, db.cluster, db.datasource, datasource)
		if err != nil {
			logInstanceError(datasource, err)
			return err
		}

		conn, err := getSqlConn(db.driverName, datasource)
		if err != nil {
			logInstanceError(datasource, err)
			return err
		}

		return query(conn, func(rows *sql.Rows) error {
			qerr = scanner(rows)
			return qerr
		}, q, args...)
	}, func(err error) bool {
		return qerr == err || db.acceptable(err)
	})
}

func (s statement) Close() error {
	return s.stmt.Close()
}

func (s statement) Exec(args ...interface{}) (sql.Result, error) {
	return execStmt(s.stmt, args...)
}

func (s statement) QueryRow(v interface{}, args ...interface{}) error {
	return queryStmt(s.stmt, func(rows *sql.Rows) error {
		return unmarshalRow(v, rows, true)
	}, args...)
}

func (s statement) QueryRowPartial(v interface{}, args ...interface{}) error {
	return queryStmt(s.stmt, func(rows *sql.Rows) error {
		return unmarshalRow(v, rows, false)
	}, args...)
}

func (s statement) QueryRows(v interface{}, args ...interface{}) error {
	return queryStmt(s.stmt, func(rows *sql.Rows) error {
		return unmarshalRows(v, rows, true)
	}, args...)
}

func (s statement) QueryRowsPartial(v interface{}, args ...interface{}) error {
	return queryStmt(s.stmt, func(rows *sql.Rows) error {
		return unmarshalRows(v, rows, false)
	}, args...)
}
