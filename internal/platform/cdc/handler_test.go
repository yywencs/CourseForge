package cdc

import (
	"context"
	"testing"

	"github.com/go-mysql-org/go-mysql/canal"
	"github.com/go-mysql-org/go-mysql/schema"
)

type recordingDocumentWriter struct {
	upserts []recordedDocument
	deletes []recordedDocument
}

type recordedDocument struct {
	index string
	id    string
	doc   any
}

func (w *recordingDocumentWriter) Upsert(
	_ context.Context,
	index string,
	id string,
	doc any,
) error {
	w.upserts = append(w.upserts, recordedDocument{index: index, id: id, doc: doc})
	return nil
}

func (w *recordingDocumentWriter) Delete(
	_ context.Context,
	index string,
	id string,
) error {
	w.deletes = append(w.deletes, recordedDocument{index: index, id: id})
	return nil
}

func TestEventHandlerSyncsCourseforgeRows(t *testing.T) {
	writer := &recordingDocumentWriter{}
	handler := NewEventHandler(&Config{ESIndexPrefix: "courseforge"}, writer)
	table := &schema.Table{
		Schema: "courseforge",
		Name:   "course",
		Columns: []schema.TableColumn{
			{Name: "id"},
			{Name: "course_name"},
		},
		PKColumns: []int{0},
	}

	err := handler.OnRow(&canal.RowsEvent{
		Table:  table,
		Action: canal.InsertAction,
		Rows:   [][]any{{uint64(101), []byte("Distributed Systems")}},
	})
	if err != nil {
		t.Fatalf("OnRow() error = %v", err)
	}
	if len(writer.upserts) != 1 {
		t.Fatalf("upserts = %#v, want one", writer.upserts)
	}
	record := writer.upserts[0]
	if record.index != "courseforge_course" || record.id != "101" {
		t.Fatalf("upsert target = %s/%s, want courseforge_course/101", record.index, record.id)
	}
	document, ok := record.doc.(map[string]any)
	if !ok {
		t.Fatalf("document type = %T, want map[string]any", record.doc)
	}
	if document["course_name"] != "Distributed Systems" ||
		document["_schema"] != "courseforge" ||
		document["_logical_table"] != "course" {
		t.Fatalf("document = %#v, want normalized courseforge metadata", document)
	}
	if handler.String() != "courseforge-cdc-handler" {
		t.Fatalf("String() = %q", handler.String())
	}
}
