# rowset

A Go library for server-side pagination, sorting, searching and filtering of SQL result sets. Designed to work with frontend table components (e.g. Angular Material `mat-table` + `mat-paginator`).

## How it works

1. The **client** sends a JSON `Request` with page index, page size, sort column, direction, and optional search/filter parameters.
2. The **server** receives the request, builds a SQL query with `WHERE`, `ORDER BY`, and `LIMIT` clauses, and executes it.
3. The **server** responds with a JSON `Response` containing the matching rows and the total row count (for the paginator).

```
┌─────────────────┐         Request (JSON)         ┌─────────────────┐
│                  │  ──────────────────────────►   │                 │
│   Angular App    │   { pageIndex, pageSize,       │    Go Server    │
│   (mat-table +   │     sort, direction,           │    (rowset)     │
│    mat-paginator)│     search, equals, ins }      │                 │
│                  │  ◄──────────────────────────   │                 │
└─────────────────┘         Response (JSON)         └─────────────────┘
                     { totalRows, rows: [...] }
```

## Install

```bash
go get github.com/delogix/rowset
```

## Structs

### Request

```go
type Request struct {
    PageIndex int               `json:"pageIndex"`
    PageSize  int               `json:"pageSize"`
    Search    map[string]string `json:"search"`    // LIKE prefix match (value%)
    Equals    map[string]string `json:"equals"`    // exact match (=)
    Ins       map[string][]int  `json:"ins"`       // IN (list of ints)
    Likes     map[string][]string `json:"likes"`   // RLIKE prefix match
    Sort      string            `json:"sort"`      // column name (json key)
    Direction string            `json:"direction"` // "asc" or "desc"
}
```

### Response

```go
type Response struct {
    TotalRows int         `json:"totalRows"`
    Rows      interface{} `json:"rows"`
}
```

## Server-side usage (Go)

### 1. Define a struct matching your SQL columns

The struct fields must have `json` tags that match the column aliases in the SELECT statement.

```go
type Person struct {
    ID        int    `json:"personId"`
    LastName  string `json:"lastName"`
    FirstName string `json:"firstName"`
    City      string `json:"city"`
}
```

### 2. Create a query and register searchable/sortable columns

```go
func (r *PersonRepo) GetPersons(req rowset.Request) (*rowset.Response, error) {
    q := rowset.NewQuery(r.db,
        "SELECT p.id as personId, p.lastname as lastName, p.firstname as firstName, p.city as city FROM t_person p WHERE p.active = ?",
        1,
    )

    // Register columns that the client is allowed to search/sort on.
    // The key is the JSON field name the client sends; the value is the actual DB column.
    q.AllowColumn("lastName",  "p.lastname")
    q.AllowColumn("firstName", "p.firstname")
    q.AllowColumn("city",      "p.city")

    return q.GetResponse(&req, Person{})
}
```

### 3. Expose via HTTP handler

```go
func (h *PersonHandler) List(w http.ResponseWriter, r *http.Request) {
    var req rowset.Request
    json.NewDecoder(r.Body).Decode(&req)

    resp, err := h.repo.GetPersons(req)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    json.NewEncoder(w).Encode(resp)
}
```

### What rowset does under the hood

Given a request `{ pageIndex: 2, pageSize: 10, search: {"lastName": "Sch"}, sort: "city", direction: "asc" }` and the query above, rowset will:

1. Parse the SQL into parts (SELECT, FROM, WHERE, GROUP BY, ORDER BY)
2. Append search filters: `WHERE p.active = ? AND p.lastname LIKE ?` with args `[1, "Sch%"]`
3. Count total matching rows: `SELECT COUNT(*) FROM t_person p WHERE p.active = ? AND p.lastname LIKE ?`
4. Append sorting: `ORDER BY p.city asc`
5. Append pagination: `LIMIT 20, 10` (page 2 × 10 items)
6. Execute the query and scan results into `[]Person`
7. Return `{ totalRows: 47, rows: [...10 items...] }`

## Client-side usage (Angular)

### 1. Create a base class for table components

```typescript
export interface RowsetRequest {
    pageIndex: number;
    pageSize?: number;
    sort?: string;
    direction?: string;
    search?: Record<string, string>;
    equals?: Record<string, string>;
}

export interface RowsetResponse {
    totalRows: number;
    rows: any;
}

export class RowsetTable {
    pageIndex = 0;
    pageSize = 10;
    sort = "";
    direction = "asc";
    tableLength = 0;
    search: Record<string, string> = {};
    equals: Record<string, string> = {};

    getRows: () => void;

    rowsetRequest(): RowsetRequest {
        return {
            pageIndex: this.pageIndex,
            pageSize: this.pageSize,
            sort: this.sort,
            direction: this.direction,
            search: this.search,
            equals: this.equals,
        };
    }

    onTablePageEvent(event: any) {
        this.pageSize = event.pageSize;
        this.pageIndex = event.pageIndex;
        this.getRows();
    }

    onTableSortEvent(event: any) {
        this.pageIndex = 0;
        this.sort = event.active;
        this.direction = event.direction;
        this.getRows();
    }

    onSearchClick() {
        this.pageIndex = 0;
        this.getRows();
    }
}
```

### 2. Build a component using RowsetTable

```typescript
@Component({
  selector: 'person-list',
  templateUrl: './person-list.component.html',
})
export class PersonListComponent extends RowsetTable implements OnInit {

  dataSource = new MatTableDataSource<Person>([]);
  displayedColumns = ['lastName', 'firstName', 'city'];
  searchGroup: FormGroup;

  constructor(
    private fb: FormBuilder,
    private personService: PersonService,
  ) {
    super();
    this.pageSize = 20;
    this.searchGroup = this.fb.group({ lastName: [], city: [] });
  }

  ngOnInit() {
    this.getRows();
  }

  getRows = () => {
    // Convert form values to search map, dropping empty values
    this.search = Object.fromEntries(
      Object.entries(this.searchGroup.value)
        .filter(([_, v]) => v !== null && v !== '')
        .map(([k, v]) => [k, String(v)])
    );

    this.personService.getPersons(this.rowsetRequest()).subscribe(
      (res: RowsetResponse) => {
        this.dataSource.data = res.rows;
        this.tableLength = res.totalRows;
      }
    );
  };
}
```

### 3. Template with mat-table and mat-paginator

```html
<div [formGroup]="searchGroup">
    <mat-form-field>
        <mat-label>Last Name</mat-label>
        <input matInput formControlName="lastName">
    </mat-form-field>
    <mat-form-field>
        <mat-label>City</mat-label>
        <input matInput formControlName="city">
    </mat-form-field>
    <button mat-button (click)="onSearchClick()">Search</button>
</div>

<mat-table [dataSource]="dataSource" matSort (matSortChange)="onTableSortEvent($event)">

    <ng-container matColumnDef="lastName">
        <mat-header-cell *matHeaderCellDef mat-sort-header>Last Name</mat-header-cell>
        <mat-cell *matCellDef="let row">{{ row.lastName }}</mat-cell>
    </ng-container>

    <ng-container matColumnDef="firstName">
        <mat-header-cell *matHeaderCellDef mat-sort-header>First Name</mat-header-cell>
        <mat-cell *matCellDef="let row">{{ row.firstName }}</mat-cell>
    </ng-container>

    <ng-container matColumnDef="city">
        <mat-header-cell *matHeaderCellDef mat-sort-header>City</mat-header-cell>
        <mat-cell *matCellDef="let row">{{ row.city }}</mat-cell>
    </ng-container>

    <mat-header-row *matHeaderRowDef="displayedColumns"></mat-header-row>
    <mat-row *matRowDef="let row; columns: displayedColumns;"></mat-row>

</mat-table>

<mat-paginator
    [length]="tableLength"
    [pageSize]="pageSize"
    [pageIndex]="pageIndex"
    [pageSizeOptions]="[10, 20, 50]"
    (page)="onTablePageEvent($event)">
</mat-paginator>
```

### 4. Service

```typescript
@Injectable({ providedIn: 'root' })
export class PersonService {
  constructor(private http: HttpClient) {}

  getPersons(data: RowsetRequest) {
    return this.http.post<RowsetResponse>('/api/persons', data);
  }
}
```

## Filter types

| Type | Request field | SQL generated | Use case |
|---|---|---|---|
| **Search** | `search: {"lastName": "Sch"}` | `lastname LIKE 'Sch%'` | Prefix search / autocomplete |
| **Equals** | `equals: {"city": "Berlin"}` | `city = 'Berlin'` | Exact match / dropdown filter |
| **Ins** | `ins: {"status": [1,2,3]}` | `status IN (1,2,3)` | Multi-select filter |
| **Likes** | `likes: {"code": ["A","B"]}` | `code RLIKE '^A|^B'` | Multi-prefix match |

## Known limitations

SQL statements with inner selects (subqueries in WHERE) are not supported:

```sql
-- Not supported
SELECT id, name FROM mytable WHERE name LIKE (SELECT name FROM othertable);
```
