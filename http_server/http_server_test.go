package http_server

import (
	"testing"
)

//func TestPSQLIsSelectOnly(t *testing.T) {
//	selectOnly, err := PSQLIsSelectOnly("SELECT 1")
//	if err != nil {
//		t.Fatal(err)
//	}
//	if !selectOnly {
//		t.Fatal("not select only")
//	}
//
//	selectOnly, err = PSQLIsSelectOnly("SELECT (SELECT 1, 2 as iuu) as heheh")
//	if err != nil {
//		t.Fatal(err)
//	}
//	if !selectOnly {
//		t.Fatal("not select only")
//	}
//
//	selectOnly, err = PSQLIsSelectOnly(`UPDATE dummy
//SET customer=subquery.customer,
//    address=subquery.address,
//    partn=subquery.partn
//FROM (SELECT address_id, customer, address, partn
//      FROM  hehe) AS subquery
//WHERE dummy.address_id=subquery.address_id;`)
//	if err != nil {
//		t.Fatal(err)
//	}
//	if selectOnly {
//		t.Fatal("select only")
//	}
//
//	selectOnly, err = PSQLIsSelectOnly(`insert into items_ver
//select * from items where item_id=2;`)
//	if err != nil {
//		t.Fatal(err)
//	}
//	if selectOnly {
//		t.Fatal("select only")
//	}
//
//	selectOnly, err = PSQLIsSelectOnly(`UPDATE a SET b = 'c' WHERE b = 'c' RETURNING *`)
//	if err != nil {
//		t.Fatal(err)
//	}
//	if selectOnly {
//		t.Fatal("select only")
//	}
//}

func TestCRDBIsSelectOnly(t *testing.T) {
	selectOnly, err := CRDBIsSelectOnly("SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	if !selectOnly {
		t.Fatal("not select only")
	}

	selectOnly, err = CRDBIsSelectOnly("SELECT (SELECT 1, 2 as iuu) as heheh")
	if err != nil {
		t.Fatal(err)
	}
	if !selectOnly {
		t.Fatal("not select only")
	}

	selectOnly, err = CRDBIsSelectOnly(`UPDATE dummy
SET customer=subquery.customer,
    address=subquery.address,
    partn=subquery.partn
FROM (SELECT address_id, customer, address, partn
      FROM  hehe) AS subquery
WHERE dummy.address_id=subquery.address_id;`)
	if err != nil {
		t.Fatal(err)
	}
	if selectOnly {
		t.Fatal("select only")
	}

	selectOnly, err = CRDBIsSelectOnly(`insert into items_ver
select * from items where item_id=2;`)
	if err != nil {
		t.Fatal(err)
	}
	if selectOnly {
		t.Fatal("select only")
	}

	selectOnly, err = CRDBIsSelectOnly(`UPDATE a SET b = 'c' WHERE b = 'c' RETURNING *`)
	if err != nil {
		t.Fatal(err)
	}
	if selectOnly {
		t.Fatal("select only")
	}

	selectOnly, err = CRDBIsSelectOnly(`SELECT somfunc() FROM wefuhw AS OF SYSTEM TIME follower_read_timestamp()`)
	if err != nil {
		t.Fatal(err)
	}
	if !selectOnly {
		t.Fatal("not select only")
	}
}
