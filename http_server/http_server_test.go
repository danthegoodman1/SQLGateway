package http_server

import (
	"testing"
)

func TestIsSelectOnly(t *testing.T) {
	selectOnly, err := IsSelectOnly("SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	if !selectOnly {
		t.Fatal("not select only")
	}

	selectOnly, err = IsSelectOnly("SELECT (SELECT 1, 2 as iuu) as heheh")
	if err != nil {
		t.Fatal(err)
	}
	if !selectOnly {
		t.Fatal("not select only")
	}

	selectOnly, err = IsSelectOnly(`UPDATE dummy
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

	selectOnly, err = IsSelectOnly(`insert into items_ver
select * from items where item_id=2;`)
	if err != nil {
		t.Fatal(err)
	}
	if selectOnly {
		t.Fatal("select only")
	}

	selectOnly, err = IsSelectOnly(`UPDATE a SET b = 'c' WHERE b = 'c' RETURNING *`)
	if err != nil {
		t.Fatal(err)
	}
	if selectOnly {
		t.Fatal("select only")
	}
}
