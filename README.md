# rowset

rowset is a library which provides functionality for data pagination for clients sending a JSON request and on the server side a GO library handling this request
mapping a DB Query to a GO Structure and marschalling to JSON.

Major concepts are:

* Marshal DB rows into GO struct 
* Limit number of rows returned
* Sorting and searching implementation

## Recent Changes


## install

    go get github.com/delogix/rowset

## issues

Sql statement with inner selects are currently not allowed

```sql
SELECT id, name FROM mytable where name like select name from othertable;
```


## usage


```sql

type Person struct {
	Id        int            `json:"personid"`
	LastName  string         `json:"lastname"`
	FirstName string         `json:"firstname"`
}


func SimpleQuery(db *sql.DB) {

    req := Request{PageIndex: 0, PageSize: 10}

	sqlStr := "select personid, firstname, lastname from t_person "
	q := NewQuery(db, sqlStr)

	
	res := q.GetResponse(&Person{}, &req)

	j, err := json.Marshal(&res)
	if err != nil {
		log.Println(err)
	}
	log.Printf("RESULT: %s", j)

}
```




