package upload

import (
	"io"
	"os"

	"github.com/materials-commons/mcstore/pkg/app"
	"github.com/materials-commons/mcstore/pkg/app/flow"
	"github.com/materials-commons/mcstore/pkg/db/dai"
	"github.com/materials-commons/mcstore/pkg/db/schema"
)

// AssemblerFactory creates new instance of an Assembler
type AssemblerFactory interface {
	Assembler(req *flow.Request, owner string) *Assembler
}

// MCDirAssemblerFactory creates new instance of an Assembler using
// app.MCDir to determine locations, and DirItemSupplier to get the
// list of Items to assemble.
type MCDirAssemblerFactory struct {
	FinisherFactory
	files dai.Files
}

// NewMCDirAssemblerFactory creates a new instance of MCDirAssemblerFactory.
func NewMCDirAssemblerFactory(ff FinisherFactory, files dai.Files) *MCDirAssemblerFactory {
	return &MCDirAssemblerFactory{
		FinisherFactory: ff,
		files:           files,
	}
}

// Assembler creates the new Assembler using a DirItemSupplier, and app.MCDir
// to determine location. Assembler will also make all the paths exist by
// calling os.MkdirAll, and creating the destination file to write.
func (f *MCDirAssemblerFactory) Assembler(req *flow.Request, owner string) *Assembler {
	itemSupplier := newDirItemSupplier(app.MCDir.UploadDir(req.UploadID()))
	newFile := schema.NewFile(req.FlowFileName, owner)
	file, err := f.files.Insert(&newFile, req.DirectoryID, req.ProjectID)
	if err != nil {
		app.Log.Error(app.Logf("Cannot insert new file into database for flow.Request: %#v", req))
		return nil
	}

	fileDir := app.MCDir.FileDir(file.ID)
	if err := os.MkdirAll(fileDir, 0700); err != nil {
		app.Log.Error(app.Logf("Cannot create path (%s) to assemble file: %s ", fileDir, file.ID))
		return nil
	}

	destination, err := os.Create(app.MCDir.FilePath(file.ID))
	if err != nil {
		app.Log.Error(app.Logf("Cannot create %s to assemble upload", app.MCDir.FilePath(file.ID)))
		return nil
	}

	return NewAssembler(itemSupplier, destination, file.ID, f.Finisher(req, f.files))
}

// A Assembler takes a list of items and assembles them.
type Assembler struct {
	ItemSupplier
	Finisher
	destination io.Writer
	fileID      string
}

// NewAssembler creates an Assembler.
func NewAssembler(itemSupplier ItemSupplier, destination io.Writer, fileID string, finisher Finisher) *Assembler {
	return &Assembler{
		ItemSupplier: itemSupplier,
		Finisher:     finisher,
		destination:  destination,
		fileID:       fileID,
	}
}

// Assemble will write the assembled items to destination. It writes
// the items in the order they were give. If it can't write
// any item, it will quit on that item and return it's error.
// If it is able to write all items then it will call Finisher.
// It only calls Finisher if it was able to successfully write
// all items. If it calls Finisher it will return its result.
func (a *Assembler) Assemble() error {
	if err := a.writeEach(); err != nil {
		return err
	}

	return a.Finish(a.fileID)
}

// writeEach attempts to write each item to destination. It
// will stop on the first item it cannot write and return
// its error.
func (a *Assembler) writeEach() error {
	items, err := a.Items()
	if err != nil {
		return err
	}

	for _, item := range items {
		if err := a.writeItem(item); err != nil {
			return err
		}
	}
	return nil
}

// writeItemTo performs the write to destination of a particular
// item. It calls copy to append the item to destination. If the
// reader returned by a item is a ReadCloser then it will call
// the close routine.
func (a *Assembler) writeItem(item Item) error {
	switch source, err := item.Reader(); {
	case err != nil:
		return err
	default:
		if closer, ok := source.(io.ReadCloser); ok {
			defer closer.Close()
		}
		_, err = io.Copy(a.destination, source)
		return err
	}
}
