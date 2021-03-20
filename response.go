package rowset

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// Response the result sent to the client
type Response struct {
	// Total recors found in result set
	TotalRows int `json:"totalRows"`
	// Items to retun to the client, max 10
	Rows interface{} `json:"rows"`
}

// GetResponse  will return the Response struct
func (t *Query) GetResponse(dest interface{}, req *Request) (*Response, error) {

	v := reflect.ValueOf(dest).Elem()
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("must pass a struct and not pointer as dest in GetResponse")
	}

	rows, err := t.GetRows(req)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rs, err := t.rowsToStruct(dest, rows)
	if err != nil {
		return nil, err
	}
	resp := Response{Rows: rs, TotalRows: t.totalRows}
	return &resp, nil
}

func (q *Query) rowsToStruct(dest interface{}, rows *sql.Rows) ([]interface{}, error) {

	columnNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	maps, err := dbFieldToStructField(dest, rows)
	if err != nil {
		return nil, err
	}

	base := reflect.TypeOf(dest).Elem()

	var rs []interface{}
	for rows.Next() {
		// clone dest
		v := reflect.New(base).Elem()

		pointers := make([]interface{}, len(columnNames))
		for i := range columnNames {
			field := v.Field(maps[i])
			if !field.IsValid() {
				return nil, fmt.Errorf("struct field position %v is not valid", i)
			}
			pointers[i] = field.Addr().Interface()
		}
		err := rows.Scan(pointers...)
		if err != nil {
			return nil, err
		}
		rs = append(rs, pointers)
	}
	return rs, nil
}

// creates a array with the field positions in the db query and
// the corresponding Struct position from the json tag
func dbFieldToStructField(dest interface{}, rows *sql.Rows) ([]int, error) {
	columnNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	seq := make([]int, len(columnNames))
	for i, name := range columnNames {
		index, err := getFieldIndexFromTag(dest, name)
		if err != nil {
			return nil, fmt.Errorf("mapping from dbfield '%s' to struct failed, %s", name, err)
		}
		seq[i] = index
	}
	return seq, nil
}

func getFieldIndexFromTag(dest interface{}, jsonName string) (int, error) {
	t := reflect.TypeOf(dest).Elem()
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
