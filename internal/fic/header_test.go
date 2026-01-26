package fic

import (
	"context"
	"testing"
)

func TestReadHeaderSuccess(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
	})
	rec, ok, err := readHeader(context.TODO(), path)
	if err != nil {
		t.Fatalf("readHeader: %v", err)
	}
	if !ok {
		t.Fatalf("expected header")
	}
	if rec.Algo != "sha256" || rec.Root != "/tmp" {
		t.Fatalf("unexpected header: %+v", rec)
	}
}

func TestReadHeaderMissing(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
	})
	_, ok, err := readHeader(context.TODO(), path)
	if err != nil {
		t.Fatalf("readHeader: %v", err)
	}
	if ok {
		t.Fatalf("expected missing header")
	}
}

func TestReadHeaderInvalidRecord(t *testing.T) {
	path := writeStateFile(t, []string{
		`{`,
	})
	if _, _, err := readHeader(context.TODO(), path); err == nil {
		t.Fatalf("expected invalid record error")
	}
}

func TestReadHeaderInvalidHeader(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":"bad"}`,
	})
	if _, _, err := readHeader(context.TODO(), path); err == nil {
		t.Fatalf("expected invalid header error")
	}
}

func TestReadHeaderVersionMismatch(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":2,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
	})
	if _, _, err := readHeader(context.TODO(), path); err == nil {
		t.Fatalf("expected version mismatch error")
	}
}
