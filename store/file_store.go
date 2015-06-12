package store

import (
	"flag"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	. "github.com/attic-labs/noms/dbg"
	"github.com/attic-labs/noms/ref"
)

var (
	dirFlag = flag.String("file-store", "", "directory to use as a FileStore")
)

type FileStore struct {
	dir string
}

func NewFileStore(dir string) FileStore {
	Chk.NotEmpty(dir)
	Chk.NoError(os.MkdirAll(dir, 0700))
	return FileStore{dir}
}

func NewFileStoreFromFlags() FileStore {
	return NewFileStore(*dirFlag)
}

func (f FileStore) Get(ref ref.Ref) (io.ReadCloser, error) {
	return os.Open(getPath(f.dir, ref))
}

func (f FileStore) Put() ChunkWriter {
	return &fileChunkWriter{
		root: f.dir,
		hash: ref.NewHash(),
	}
}

type fileChunkWriter struct {
	root   string
	file   *os.File
	writer io.Writer
	hash   hash.Hash
}

func (w *fileChunkWriter) Write(data []byte) (int, error) {
	if w.file == nil {
		f, err := ioutil.TempFile(os.TempDir(), "")
		Chk.NoError(err)
		w.file = f
		w.writer = io.MultiWriter(f, w.hash)
	}
	return w.writer.Write(data)
}

func (w *fileChunkWriter) Ref() (ref.Ref, error) {
	ref := ref.FromHash(w.hash)
	p := getPath(w.root, ref)
	err := os.MkdirAll(path.Dir(p), 0700)
	Chk.NoError(err)

	err = os.Rename(w.file.Name(), p)
	if err != nil {
		Chk.True(os.IsExist(err))
	}
	return ref, nil
}

func (w *fileChunkWriter) Close() error {
	w.file.Close()
	os.Remove(w.file.Name())
	w.file = nil
	return nil
}

func getPath(root string, ref ref.Ref) string {
	s := ref.String()
	Chk.True(strings.HasPrefix(s, "sha1"))
	return path.Join(root, "sha1", s[5:7], s[7:9], s)
}
