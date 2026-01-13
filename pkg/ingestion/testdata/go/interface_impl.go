package sample

// Reader defines a read interface.
type Reader interface {
	Read(p []byte) (n int, err error)
}

// Writer defines a write interface.
type Writer interface {
	Write(p []byte) (n int, err error)
}

// ReadWriter combines Reader and Writer.
type ReadWriter interface {
	Reader
	Writer
}
