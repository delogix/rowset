package rowset

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"reflect"
)

// Response the result sent to the client
type Response struct {
	// Total recors found in result set
	TotalRows int `json:"totalRows"`
	// Items to retun to the client, max 10
	Rows interface{} `json:"rows"`
}

// GetResponse  will return the Response struct
func (t *Query) GetResponse(dest interface{}, req *Request) Response {

	v := reflect.ValueOf(dest).Elem()
	if v.Kind() != reflect.Struct {
		log.Fatal("must pass a struct and not pointer as dest in GetResponse")
	}

	rows, err := t.GetRows(req)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	rs := t.rowsToStruct(dest, rows)
	resp := Response{Rows: rs, TotalRows: t.totalRows}
	return resp
}

func (q *Query) rowsToStruct(dest interface{}, rows *sql.Rows) []interface{} {

	columnNames, err := rows.Columns()
	if err != nil {
		log.Fatal(err)
	}

	maps, err := dbFieldToStructField(dest, rows)
	if err != nil {
		log.Fatal(err)
	}

	base := reflect.TypeOf(dest).Elem()

	var rs []interface{}
	for rows.Next() {
		// clone dest
		v := reflect.New(base).Elem()

		pointers := make([]interface{}, len(columnNames))
		for i, dbColName := range columnNames {
			field := v.Field(maps[i])
			if !field.IsValid() {
				log.Fatal("field is not valid ", dbColName)
			}
			pointers[i] = field.Addr().Interface()
		}
		err := rows.Scan(pointers...)
		if err != nil {
			log.Fatal(err)
		}
		rs = append(rs, pointers)
	}
	return rs
}

// creates a array with the field positions in the db query and
// the corresponding Struct position from the json tag
func dbFieldToStructField(dest interface{}, rows *sql.Rows) ([]int, error) {
	columnNames, err := rows.Columns()
	if err != nil {
		log.Fatal(err)
	}
	seq := make([]int, len(columnNames))
	for i, name := range columnNames {
		index, err := getFieldIndexFromTag(dest, name)
		if err != nil {
			return nil, err
		}
		seq[i] = index
	}
	return seq, nil
}

func getFieldIndexFromTag(dest interface{}, jsonName string) (int, error) {
	t := reflect.TypeOf(dest).Elem()
	for i := 0; i < t.NumField(); i++ {
		if name, ok := t.Field(i).Tag.Lookup("json"); ok {
			if name == jsonName {
				return i, nil
			}
		}
	}
	s := fmt.Sprintf("json name '%s' not found in struct ", jsonName)
	return 0, errors.New(s)
}
