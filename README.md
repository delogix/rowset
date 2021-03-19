# rowset

rowset is a library which provides ...

Major additional concepts are:

* Marshal rows into struct 
* Limit number of rows returned
* Sort and search

## Recent Changes


## install

    go get github.com/delogix/rowset

## issues

Sql statement with inner selects are currently not allowed

```sql
SELECT id, name FROM mytable where name like select name from othertable;
```


## usage


