package rowset

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// Response the result sent sended to the client
type Response struct {
	// Total records found in the result set
	TotalRows int `json:"totalRows"`
	// Items returned to the client
	Rows interface{} `json:"rows"`
}

// GetResponse  will return the Response struct
func (q *Query) GetResponseFromStruct(req *Request, arrStruct interface{}) (*Response, error) {
	resp := Response{Rows: arrStruct, TotalRows: q.totalRows}
	return &resp, nil
}

// GetResponse  will return the Response struct
func (q *Query) GetResponse(req *Request, dest interface{}) (*Response, error) {
	q.dest = dest

	rows, err := q.GetRows(req)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rs, err := q.rowsToStruct(rows)
	if err != nil {
		return nil, err
	}
	resp := Response{Rows: rs, TotalRows: q.totalRows}
	return &resp, nil
}

func (q *Query) rowsToStruct(rows *sql.Rows) ([]interface{}, error) {

	columnNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	maps, err := q.dbFieldToStructField(rows)
	if err != nil {
		return nil, err
	}

	t := reflect.TypeOf(q.dest)
	var rs []interface{}

	for rows.Next() {
		// clone dest
		n := reflect.New(t).Elem()
		pointers := make([]interface{}, len(columnNames))

		for i := range columnNames {
			pointers[i] = n.Field(maps[i]).Addr().Interface()
		}
		err := rows.Scan(pointers...)
		if err != nil {
			return nil, err
		}
		rs = append(rs, n.Interface())
	}
	return rs, nil
}

// creates a array with the field positions in the db query and
// the corresponding Struct position from the json tag
func (q *Query) dbFieldToStructField(rows *sql.Rows) ([]int, error) {
	columnNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	seq := make([]int, len(columnNames))
	for i, name := range columnNames {
		index, err := q.getFieldIndexFromTag(name)
		if err != nil {
			return nil, fmt.Errorf("mapping from dbfield '%s' to struct failed, %s", name, err)
		}
		seq[i] = index
	}
	return seq, nil
}

func (q *Query) getFieldIndexFromTag(jsonName string) (int, error) {
	t := reflect.TypeOf(q.dest)
	for i := 0; i < t.NumField(); i++ {
		if name, ok := t.Field(i).Tag.Lookup("json"); ok {
			names := strings.Split(name, ",")
			for _, n := range names {
				if n == jsonName {
					return i, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("tag json:\"%s\" not found in struct", jsonName)
}
