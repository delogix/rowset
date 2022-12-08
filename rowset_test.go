package rowset

import (
	"database/sql"
	"encoding/json"
	"log"
	"os"

	"testing"

	"github.com/go-sql-driver/mysql"
)

var db *sql.DB

func init() {
	ConnectDB()
	dropTables()
	createTables()
	loadFixture()
}

func ConnectDB() {
	var err error

	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		// log.Fatalf("var MYSQL_DSN is not configured in your shell environment %v", dsn)
		dsn = "root:pass@tcp(127.0.0.1:3306)/terra_pflege?parseTime=true"
	}

	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("mysql err", err)
	}
	err = db.Ping()
	if err != nil {
		db.Close()
		log.Fatal(err)
	}
}
func createTables() {
	dbExec(`
	CREATE TABLE t_person (
		personid int,
		firstname text,
		lastname text,
		title text NULL,
		rating double NULL,
		birthday date NULL,
		gender varchar(100) NULL,
		created timestamp default now()
	)`)

}
func dropTables() {
	dbExec(`DROP TABLE IF EXISTS t_person`)
}

func loadFixture() {
	dbExec(`
	INSERT INTO t_person(personid,firstname,lastname,title,rating,birthday) VALUES
					(1,'don','joe','employee','1.5','1980-10-01'),
					(2,'mary','joe','wife','2.5','1985-11-05'),
					(3,'peter','pan','comic','1.0','1970-01-01'),
					(4,'super','man',NULL,NULL,NULL);
    `)

}
func dbExec(stmt string) {
	_, err := db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}
}

type Person struct {
	Id        int     `json:"personid"`
	LastName  string  `json:"lastname"`
	FirstName string  `json:"firstname"`
	Title     string  `json:"title"`
	Rating    float32 `json:"rating"`
	Gender    string
}

func (p *Person) Clone() *Person {
	return &Person{}
}

type NullDate struct {
	mysql.NullTime
}

func NewPerson() interface{} {
	return new(Person)
}

func TestSimpleQuery(t *testing.T) {

	//person := Person{}

	sqlStr := "select personid, firstname, lastname from t_person "
	q := NewQuery(db, sqlStr)

	req := Request{PageIndex: 0, PageSize: 10}

	p := Person{}
	res, err := q.GetResponse(&req, p)
	if err != nil {
		t.Error(err)
	}

	j, err1 := json.Marshal(&res)
	if err1 != nil {
		t.Error(err1)
	}
	log.Printf("test RESULT: %s", j)

}

func TestSimpleInsQuery(t *testing.T) {

	//person := Person{}

	sqlStr := "select personid, firstname, lastname from t_person p "
	q := NewQuery(db, sqlStr)
	q.AllowColumn("pid", "p.personid")

	req := Request{PageIndex: 0, PageSize: 10}

	req.Ins = map[string][]int{"pid": {2, 3}}

	p := Person{}
	res, err := q.GetResponse(&req, p)
	if err != nil {
		t.Error(err)
	}

	j, err1 := json.Marshal(&res)
	if err1 != nil {
		t.Error(err1)
	}
	log.Printf("test RESULT: %s", j)
	log.Println("End")

}

func TestFieldInStructJsonTagNotFound(t *testing.T) {

	person := Person{}

	sqlStr := "select personid, firstname, lastname, gender from t_person "
	q := NewQuery(db, sqlStr)

	req := Request{PageIndex: 0, PageSize: 10}

	res, err := q.GetResponse(&req, person)
	if err == nil {
		t.Error("expected mapping gender error! but no error found")
	}

	_, err = json.Marshal(&res)
	if err != nil {
		t.Error(err)
	}
	//log.Printf("RESULT: %s", j)

}

func TestNullFieldInStruct(t *testing.T) {

	person := Person{}

	sqlStr := "select personid, firstname, lastname, title from t_person "
	q := NewQuery(db, sqlStr)

	req := Request{PageIndex: 0, PageSize: 10}

	res, err := q.GetResponse(&req, person)
	if err != nil {
		t.Error(err)
	}

	_, err = json.Marshal(&res)
	if err != nil {
		t.Error(err)
	}
	//log.Printf("RESULT: %s", j)

}

func TestWrongSQL(t *testing.T) {

	person := Person{}

	sqlStr := "select xxx from t_person "
	q := NewQuery(db, sqlStr)

	req := Request{PageIndex: 0, PageSize: 10}

	res, err := q.GetResponse(&req, person)
	if err != nil {
		t.Error(err)
	}

	_, err = json.Marshal(&res)
	if err != nil {
		t.Error(err)
	}
	//log.Printf("RESULT: %s", j)
}

func TestPageIndex(t *testing.T) {

	person := Person{}

	sqlStr := "select personid, firstname, lastname, title from t_person "
	q := NewQuery(db, sqlStr)

	req := Request{PageIndex: 10, PageSize: 10}

	res, err := q.GetResponse(&req, person)
	if err != nil {
		t.Error(err)
	}

	_, err = json.Marshal(&res)
	if err != nil {
		t.Error(err)
	}
	//log.Printf("RESULT: %s", j)

}

func TestSort(t *testing.T) {

	person := Person{}

	sqlStr := "select personid, firstname, lastname from t_person p "
	q := NewQuery(db, sqlStr)
	q.AllowColumn("lastname", "p.lastname")

	req := Request{PageIndex: 0, PageSize: 10, Sort: "lastname", Direction: "desc"}

	res, err := q.GetResponse(&req, person)
	if err != nil {
		t.Error(err)
	}

	_, err = json.Marshal(&res)
	if err != nil {
		t.Error(err)
	}
	//log.Printf("RESULT: %s", j)

}

func TestSearch(t *testing.T) {

	person := Person{}

	sqlStr := "select personid, firstname, lastname from t_person p "
	q := NewQuery(db, sqlStr)
	q.Search("lastname", "pan")

	req := Request{PageIndex: 0, PageSize: 10, Sort: "lastname", Direction: "desc"}

	res, err := q.GetResponse(&req, person)
	if err != nil {
		t.Error(err)
	}

	_, err = json.Marshal(&res)
	if err != nil {
		t.Error(err)
	}
	//log.Printf("RESULT: %s", j)

}

func TestLikes(t *testing.T) {

	person := Person{}

	sqlStr := "select personid, firstname, lastname from t_person "
	q := NewQuery(db, sqlStr)
	q.AllowColumn("lastname", "lastname")

	req := Request{PageIndex: 0, PageSize: 10, Sort: "lastname", Direction: "desc"}
	req.Likes = map[string][]string{"lastname": {"da", "de", "di"}}

	res, err := q.GetResponse(&req, person)
	if err != nil {
		t.Error(err)
	}

	_, err = json.Marshal(&res)
	if err != nil {
		t.Error(err)
	}
	//log.Printf("RESULT: %s", j)

}
