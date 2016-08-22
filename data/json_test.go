package data_test

import (
	"fmt"

	"github.com/dailyburn/ratchet/data"
)

type testStruct struct {
	A int
	B int
}

func ExampleNewJSON() {
	t := testStruct{A: 1, B: 2}

	d, _ := data.NewJSON(t)

	fmt.Println(string(d))
	// Output: {"A":1,"B":2}
}

func ExampleParseJSON() {
	d := []byte(`{"A":1,"B":2}`)
	t := testStruct{}
	data.ParseJSON(d, &t)

	fmt.Println(fmt.Sprintf("%+v", t))
	// Output: {A:1 B:2}
}

func ExampleObjectsFromJSON() {
	d := []byte(`[{"One":1},
		      {"Two":2}]`)

	objects, _ := data.ObjectsFromJSON(d)

	fmt.Println(fmt.Sprintf("%+v", objects))
	// Output: [map[One:1] map[Two:2]]
}

func ExampleObjectsFromJSONIsNull() {
	d := []byte("null")

	objects, _ := data.ObjectsFromJSON(d)

	fmt.Println(fmt.Sprintf("%+v", objects))
	// Output: []
}

func ExampleJSONFromHeaderAndRows() {
	header := []string{"A", "B", "C"}
	rows := [][]interface{}{
		[]interface{}{1, 2, 3},
		[]interface{}{4, 5, 6},
	}

	d, _ := data.JSONFromHeaderAndRows(header, rows)

	fmt.Println(fmt.Sprintf("%+v", string(d)))
	// Output: [{"A":1,"B":2,"C":3},{"A":4,"B":5,"C":6}]
}
