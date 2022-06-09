package sqlx

import "github.com/go-sql-driver/mysql"

const (
	mysqlDriverName           = "mysql"
	duplicateEntryCode uint16 = 1062
)

// NewMysql returns a mysql connection.
func NewMysql(datasource map[string]string, cluster bool, opts ...SqlOption) SqlConn {
	opts = append(opts, withMysqlAcceptable())
	return NewSqlConn(mysqlDriverName, datasource, cluster, opts...)
}

func mysqlAcceptable(err error) bool {
	if err == nil {
		return true
	}

	myerr, ok := err.(*mysql.MySQLError)
	if !ok {
		return false
	}

	switch myerr.Number {
	case duplicateEntryCode:
		return true
	default:
		return false
	}
}

func withMysqlAcceptable() SqlOption {
	return func(conn *commonSqlConn) {
		conn.accept = mysqlAcceptable
	}
}
