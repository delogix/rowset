package rowset

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// package-level compiled regexes — avoids recompiling on every call
var (
	reBlank    = regexp.MustCompile(`[[:blank:]]`)
	reSpace    = regexp.MustCompile(`[[:space:]]{2,}`)
	reSelect   = regexp.MustCompile(`(?i)select(.+)from(.+)`)
	reOrderBy  = regexp.MustCompile(`(?i)(.+)order by(.+)`)
	reGroupBy  = regexp.MustCompile(`(?i)(.+)group by(.+)`)
	reWhere    = regexp.MustCompile(`(?i)(.+)where(.+)`)
)

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
	// Ins is a map from external variable name to array of integers
	Ins map[string][]int `json:"ins"`
	// Likes is a map from external variable name to array of strings (prefix match)
	Likes map[string][]string `json:"likes"`
	// Equals is a map from external variable name to an exact match value
	Equals map[string]string `json:"equals"`
	// Sort is the order by (external variable name) in the sql statement
	Sort string `json:"sort"`
	// Direction sort direction: asc or desc
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
	q.Allows = make(map[string]string)
	return &q
}

// SetArgs set arguments for the sql statement
func (q *Query) SetArgs(args ...interface{}) {
	q.Args = args
}

// Search adds a LIKE filter for the given db field and value
func (q *Query) Search(fieldname string, value string) {
	q.filters = append(q.filters, fieldname)
	q.Args = append(q.Args, fmt.Sprint(value, "%"))
}

// AllowColumn registers a db column as searchable/sortable under the given JSON name.
// Use the table prefix when needed, e.g. q.AllowColumn("personName", "p.name")
func (q *Query) AllowColumn(jsonName string, dbField string) {
	q.Allows[jsonName] = dbField
}

// GetRows returns the paged sql.Rows result
func (q *Query) GetRows(req *Request) (*sql.Rows, error) {
	if q.SqlStr == "" {
		return nil, fmt.Errorf("statement is not set")
	}
	q.normalizeStatement()
	q.addSearch(req.Search)
	q.setWhere()
	q.setIns(req)
	q.setLikes(req)
	q.setEquals(req)

	err := q.setTotalRows()
	if err != nil {
		return nil, err
	}

	// Column sorting — direction is validated to prevent SQL injection
	if req.Sort != "" {
		dir := strings.ToLower(req.Direction)
		if dir != "asc" && dir != "desc" {
			dir = "asc"
		}
		if dbcol, ok := q.Allows[req.Sort]; ok {
			q.stmt.orderBy = dbcol
			q.stmt.direction = dir
		}
	}

	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 10
	}
	from := req.PageIndex * pageSize
	stm := q.getStatement()
	stm = fmt.Sprintf("%s LIMIT %d,%d", stm, from, pageSize)

	rows, err := q.db.Query(stm, q.Args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// normalizeStatement collapses all whitespace runs to a single space
func (q *Query) normalizeStatement() {
	s := reBlank.ReplaceAllString(q.SqlStr, " ")
	s = reSpace.ReplaceAllString(s, " ")
	q.SqlStr = strings.TrimSpace(s)
}

// breakSqlInParts splits the SQL string into its structural parts.
// Uses case-insensitive matching so both upper- and lower-case SQL keywords are handled.
func (q *Query) breakSqlInParts() {
	more := ""
	q.stmt.fields, more = findMatchRe(q.SqlStr, reSelect)
	more, q.stmt.orderBy = findMatchRe(more, reOrderBy)
	more, q.stmt.groupBy = findMatchRe(more, reGroupBy)
	more, q.stmt.where = findMatchRe(more, reWhere)
	q.stmt.from = more
}

func findMatchRe(haystack string, re *regexp.Regexp) (string, string) {
	x := re.FindStringSubmatch(haystack)
	if x != nil && x[2] != "" {
		return x[1], x[2]
	}
	return haystack, ""
}

func (q *Query) setWhere() {
	q.breakSqlInParts()
	if len(q.filters) > 0 {
		f := make([]string, len(q.filters))
		for i, filter := range q.filters {
			f[i] = filter + " like ? "
		}
		where := strings.Join(f, " and ")
		if q.stmt.where != "" {
			q.stmt.where = q.stmt.where + " and " + where
		} else {
			q.stmt.where = where
		}
	}
}

func (q *Query) setIns(req *Request) {
	for jsonName, ids := range req.Ins {
		if dbcol, ok := q.Allows[jsonName]; ok {
			if len(ids) > 0 {
				placeholders := make([]string, len(ids))
				for i, id := range ids {
					placeholders[i] = "?"
					q.Args = append(q.Args, id)
				}
				in := dbcol + " in (" + strings.Join(placeholders, ",") + ")"
				q.stmt.where = appendWhere(q.stmt.where, in)
			}
		}
	}
}

// setLikes builds a RLIKE condition for each allowed column.
// User-supplied values are escaped with regexp.QuoteMeta to prevent regex injection.
func (q *Query) setLikes(req *Request) {
	for jsonName, data := range req.Likes {
		if dbcol, ok := q.Allows[jsonName]; ok {
			if len(data) > 0 {
				escaped := make([]string, len(data))
				for i, s := range data {
					escaped[i] = regexp.QuoteMeta(s)
				}
				likeStr := strings.Join(escaped, "|^")
				like := dbcol + " rlike '^" + likeStr + "' "
				if q.stmt.where != "" {
					q.stmt.where = q.stmt.where + " and " + like
				} else {
					q.stmt.where = like
				}
			}
		}
	}
}

func (q *Query) setEquals(req *Request) {
	for jsonName, val := range req.Equals {
		if dbcol, ok := q.Allows[jsonName]; ok {
			if val != "" {
				q.stmt.where = appendWhere(q.stmt.where, dbcol+" = ?")
				q.Args = append(q.Args, val)
			}
		}
	}
}

func appendWhere(existing, clause string) string {
	if existing != "" {
		return existing + " and " + clause
	}
	return clause
}


func (q *Query) getCountStatement() string {
	// If the query has a GROUP BY, wrapping in a subquery gives the correct row count.
	// Without wrapping, COUNT(*) would count groups instead of total rows.
	if q.stmt.groupBy != "" {
		return "select count(*) from (" + q.getBaseStatement() + ") as _count"
	}
	sql := "select count(*) from " + q.stmt.from
	if q.stmt.where != "" {
		sql += " where " + q.stmt.where
	}
	return sql
}

// getBaseStatement returns the query without ORDER BY and LIMIT — used for counting
func (q *Query) getBaseStatement() string {
	sql := "select " + q.stmt.fields + " from " + q.stmt.from
	if q.stmt.where != "" {
		sql += " where " + q.stmt.where
	}
	if q.stmt.groupBy != "" {
		sql += " group by " + q.stmt.groupBy
	}
	return sql
}

// getStatement assembles the full SQL string from its parsed parts.
// Each keyword clause is preceded by an explicit space to avoid concatenation errors.
func (q *Query) getStatement() string {
	sql := "select " + q.stmt.fields + " from " + q.stmt.from
	if q.stmt.where != "" {
		sql += " where " + q.stmt.where
	}
	if q.stmt.groupBy != "" {
		sql += " group by " + q.stmt.groupBy
	}
	if q.stmt.orderBy != "" {
		sql += " order by " + q.stmt.orderBy
		if q.stmt.direction != "" {
			sql += " " + q.stmt.direction
		}
	}
	return sql
}

// addSearch maps search keys to allowed db columns and queues LIKE filters
func (q *Query) addSearch(search map[string]string) {
	for k, v := range search {
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
