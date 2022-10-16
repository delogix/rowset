package rowset

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

type Record func() interface{}

// Query  saves the SQL statement and arguments
type Query struct {
	// Sql the original statement
	SqlStr string
	// args are the arguments that will be used in the sql statement
	Args []interface{}
	// Allows are the allowed public names that can be used in the search and in the order statements
	// this is a map from public column names to db column names
	Allows map[string]string
	// filters is a array of where fields that will be added to the original statement
	filters []string
	// database instance
	db *sql.DB
	// the sql statement breaked in parts
	stmt statement
	// total rows affected by the current statement
	totalRows int
	// the struct to use for reflection when scanning the DB resultset
	dest interface{}
}

// Request have the parameters used to build the page number (pageIndex) requested
type Request struct {
	// Page is the current page number
	PageIndex int `json:"pageIndex"`
	// Number of item to show in one page
	PageSize int `json:"pageSize"`
	// Search is a map from external variable name and search value
	Search map[string]string `json:"search"`
	// Search is a map from external variable name and search value
	Ins map[string][]int `json:"ins"`
	// Sort is the order by ( external variable name) in the sql statement
	Sort string `json:"sort"`
	// Sort direction asc,desc
	Direction string `json:"direction"`
}

// the sql statement breaked in parts
type statement struct {
	fields    string
	from      string
	where     string
	orderBy   string
	groupBy   string
	direction string
}

func NewQuery(db *sql.DB, sqlStr string, args ...interface{}) *Query {
	q := Query{SqlStr: sqlStr, Args: args, db: db}
	//q.stmt = statement{fields: "", from: "", where: "", orderBy: "", groupBy: "", direction: ""}
	q.Allows = make(map[string]string)
	return &q
}

// SetArgs set arguments for the sql statement
func (q *Query) SetArgs(args ...interface{}) {
	q.Args = args
}

// Filter adds a search filter to the statement
func (q *Query) Search(fieldname string, value string) {
	q.filters = append(q.filters, fieldname)
	q.Args = append(q.Args, fmt.Sprint(value, "%"))
}

// AllowColumn is a dbField that is allowed to be searched or sorted
// the name of the dbField should not be a alias, use the prefix if needed.
// example:  q.Allows["personname"] = "p.name"
func (q *Query) AllowColumn(jsonName string, dbField string) {
	q.Allows[jsonName] = dbField
}

// GetRows - Returns the response for paging and the db record set
// the calling function of GetRows should close the DB Query
func (q *Query) GetRows(req *Request) (*sql.Rows, error) {
	if q.SqlStr == "" {
		return nil, fmt.Errorf("statement is not set")
	}
	// remove white spaces
	q.normalizeStatement()
	q.addSearch(req.Search)
	// rebuild statement with where filters
	q.setWhere()
	q.setIns(req)

	err := q.setTotalRows()
	if err != nil {
		return nil, err
	}

	// Column Sorting
	if req.Sort != "" && req.Direction != "" {
		if dbcol, ok := q.Allows[req.Sort]; ok {
			q.stmt.orderBy = dbcol
			q.stmt.direction = req.Direction
		}
	}

	// build sql statement with LIMIT
	from := req.PageIndex * req.PageSize
	stm := q.getStatement()
	stm = fmt.Sprintf("%s LIMIT %d,%d", stm, from, req.PageSize)

	// Get Rows, looping the rows will be done in the calling function or by calling GetResponse instead of GetRows
	rows, err := q.db.Query(stm, q.Args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// remove tabs and whitespaces from the original statement
func (q *Query) normalizeStatement() {
	sql := []byte(q.SqlStr)
	// remove tabs
	blanks := regexp.MustCompile("[[:blank:]]")
	sql = blanks.ReplaceAll(sql, []byte(" "))
	//remove white spaces
	tabs := regexp.MustCompile("[[:space:]]{2,}")
	sql = tabs.ReplaceAll(sql, []byte(" "))
	// byte to string
	q.SqlStr = string(sql[:])
}

// break the sql string in query parts
func (q *Query) breakSqlInParts() {
	more := ""
	q.stmt.fields, more = findMatch(q.SqlStr, "select(.+)from(.+)")
	more, q.stmt.orderBy = findMatch(more, "(.+)order by(.+)")
	more, q.stmt.groupBy = findMatch(more, "(.+)group by(.+)")
	more, q.stmt.where = findMatch(more, "(.+)where(.+)")
	q.stmt.from = more
}

func findMatch(haystack string, match string) (string, string) {
	reg := regexp.MustCompile(match)
	x := reg.FindStringSubmatch(haystack)
	if x != nil && x[2] != "" {
		return x[1], x[2]
	}
	return haystack, ""
}

func (q *Query) setWhere() {
	q.breakSqlInParts()
	//  add where clause if filters found
	if len(q.filters) > 0 {
		f := make([]string, len(q.filters))
		for i, filter := range q.filters {
			f[i] = filter + " like ? "
		}
		// join array to string
		where := strings.Join(f, " and ")
		if q.stmt.where != "" {
			q.stmt.where = q.stmt.where + " and " + where
		} else {
			q.stmt.where = where
		}
	}

}

func (q *Query) setIns(req *Request) {
	if len(req.Ins) > 0 {
		for fieldname, ids := range req.Ins {
			if len(ids) > 0 {
				insStr := q.arrayToString(ids, ",")
				in := fieldname + " in ( " + insStr + ")"

				if q.stmt.where != "" {
					q.stmt.where = q.stmt.where + " and " + in
				} else {
					q.stmt.where = in
				}
			}
		}
	}
}

func (q *Query) arrayToString(a []int, delim string) string {
	return strings.Trim(strings.Replace(fmt.Sprint(a), " ", delim, -1), "[]")
}

func (q *Query) getCountStatement() string {
	sql := "select count(*) from " + q.stmt.from
	if q.stmt.where != "" {
		sql += " where " + q.stmt.where
	}
	return sql
}

func (q *Query) getStatement() string {
	sql := "select " + q.stmt.fields + "from " + q.stmt.from
	if q.stmt.where != "" {
		sql += "where " + q.stmt.where
	}
	if q.stmt.groupBy != "" {
		sql += "group by " + q.stmt.groupBy
	}
	if q.stmt.orderBy != "" {
		sql += "order by " + q.stmt.orderBy
		if q.stmt.direction != "" {
			sql += " " + q.stmt.direction
		}
	}
	return sql
}

// AddSearch - add in the where array all values that are in search and are not empty
func (q *Query) addSearch(search map[string]string) {
	for k, v := range search {
		// get the db column name from the allowed Allows MAP
		if dbcol, ok := q.Allows[k]; ok {
			if v != "" {
				q.Search(dbcol, v)
			}
		}
	}
}

func (q *Query) setTotalRows() error {
	var count int
	sqlCount := q.getCountStatement()
	err := q.db.QueryRow(sqlCount, q.Args...).Scan(&count)
	if err != nil {
		return err
	}
	q.totalRows = count
	return nil
}
