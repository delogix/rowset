# rowset

rowset is a library which provides functionality for data pagination with search capabilities on the clients like Angular and executing a SQL Query on the server
* client sends a JSON rowset.Request 
* Server handles the request and executing the SQL Query with search parameters and pagination
* Server responses with a JSON rowset.Response

Major concepts are:

* Marshal DB rows into GO struct 
* Limit number of rows returned
* Sorting and searching implementation

## Recent Changes
15 October 2022


## install

    go get github.com/delogix/rowset

## issues

Sql statement with inner selects are currently not allowed

```sql
SELECT id, name FROM mytable where name like select name from othertable;
```

## struct

```go

// Request have the parameters used to build the page number (pageIndex) requested
type Request struct {
	PageIndex   int                 `json:"pageIndex"`
	PageSize    int                 `json:"pageSize"`
	Search      map[string]string   `json:"search"`
	Ins 		map[string][]int 	`json:"ins"`
	Sort        string              `json:"sort"`
	Direction   string              `json:"direction"`
}


// Response the result sent sended to the client
type Response struct {
	TotalRows   int             `json:"totalRows"`
	Rows        interface{}     `json:"rows"`
}
```



## usage


```go

type Person struct {
	Id        int            `json:"personid"`
	LastName  string         `json:"lastname"`
	FirstName string         `json:"firstname"`
}


func SimpleExample(db *sql.DB) {

    req := Request{PageIndex: 0, PageSize: 10}
    
    q := NewQuery(db, "select personid, firstname, lastname from t_person")

    res := q.GetResponse( &req, Person{} )

    j, err := json.Marshal(&res)
    if err != nil {
        log.Println(err)
    }
    log.Printf("RESULT: %s", j)

}
```




