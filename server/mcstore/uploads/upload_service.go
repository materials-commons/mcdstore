package uploads

import (
	"path/filepath"

	"fmt"

	"crypto/md5"

	r "github.com/dancannon/gorethink"
	"github.com/materials-commons/gohandy/file"
	"github.com/materials-commons/mcstore/pkg/app"
	"github.com/materials-commons/mcstore/pkg/app/flow"
	"github.com/materials-commons/mcstore/pkg/db/dai"
	"github.com/materials-commons/mcstore/pkg/db/schema"
)

var _ = fmt.Println

// A UploadRequest contains the block to upload and the
// information required to write that block.
type UploadRequest struct {
	*flow.Request
}

type UploadStatus struct {
	FileID string
	Done   bool
}

// UploadService takes care of uploading blocks and constructing the
// file when all blocks have been uploaded.
type UploadService interface {
	Upload(req *UploadRequest) (*UploadStatus, error)
}

// uploadService is an implementation of UploadService.
type uploadService struct {
	tracker     *blockTracker
	files       dai.Files
	uploads     dai.Uploads
	dirs        dai.Dirs
	writer      requestWriter
	requestPath requestPath
	fops        file.Operations
}

// NewUploadService creates a new idService that connects to the database using
// the given session.
func NewUploadService(session *r.Session) *uploadService {
	return &uploadService{
		tracker:     requestBlockTracker,
		files:       dai.NewRFiles(session),
		uploads:     dai.NewRUploads(session),
		dirs:        dai.NewRDirs(session),
		writer:      &blockRequestWriter{},
		requestPath: &mcdirRequestPath{},
		fops:        file.OS,
	}
}

// Upload performs uploading a block and constructing the file
// after all blocks have been uploaded.
func (s *uploadService) Upload(req *UploadRequest) (*UploadStatus, error) {
	dir := s.requestPath.dir(req.Request)
	id := req.UploadID()

	if !s.tracker.idExists(id) {
		return nil, app.ErrInvalid
	}

	if err := s.writeBlock(dir, req); err != nil {
		app.Log.Errorf("Writing block %d for request %s failed: %s", req.FlowChunkNumber, id, err)
		return nil, err
	}

	uploadStatus := &UploadStatus{}

	if s.tracker.done(id) {
		if file, err := s.assemble(req, dir); err != nil {
			app.Log.Errorf("Assembly failed for request %s: %s", req.FlowIdentifier, err)
			// Assembly failed. If file isn't nil then we need to cleanup state.
			if file != nil {
				if err := s.cleanup(req, file.ID); err != nil {
					app.Log.Errorf("Attempted cleanup of failed assembly %s errored with: %s", req.FlowIdentifier, err)
				}
			}
			return nil, err
		} else {
			uploadStatus.FileID = file.ID
			uploadStatus.Done = true
		}

	}
	return uploadStatus, nil
}

// writeBlock will write the request block and update state information
// on the block only if this block hasn't already been written.
func (s *uploadService) writeBlock(dir string, req *UploadRequest) error {
	id := req.UploadID()
	var err error
	if !s.tracker.isBlockSet(id, int(req.FlowChunkNumber)) {
		s.tracker.withWriteLock(id, func(b *blockTrackerEntry) {
			err = s.writer.write(dir, req.Request)
		})
		if err == nil {
			s.tracker.addToHash(id, req.Chunk)
			s.tracker.setBlock(id, int(req.FlowChunkNumber))
		}
	}
	return err
}

// assemble moves the upload file to its proper location, creates a database entry
// and take care of all book keeping tasks to make the file accessible.
func (s *uploadService) assemble(req *UploadRequest, dir string) (*schema.File, error) {
	// Look up the upload
	upload, err := s.uploads.ByID(req.FlowIdentifier)
	if err != nil {
		return nil, err
	}

	// Create file entry in database
	file, err := s.createFile(req, upload)
	if err != nil {
		app.Log.Errorf("Assembly failed for request %s, couldn't create file in database: %s", req.FlowIdentifier, err)
		return nil, err
	}

	// Check if this is an upload matching a file that has already been uploaded. If it isn't
	// then copy over the data. If it is, then there isn't any uploaded data to copy over.
	if !upload.IsExisting {
		// Create on disk entry to write chunks to
		if err := s.createDest(file.ID); err != nil {
			app.Log.Errorf("Assembly failed for request %s, couldn't create file on disk: %s", req.FlowIdentifier, err)
			return file, err
		}

		// Move file
		uploadDir := s.requestPath.dir(req.Request)
		s.fops.Rename(filepath.Join(uploadDir, req.UploadID()), app.MCDir.FilePath(file.ID))
	}

	// Finish updating the file state.
	finisher := newFinisher(s.files, s.dirs)
	checksum := s.determineChecksum(req, upload)
	if err := finisher.finish(req, file.ID, checksum, upload); err != nil {
		app.Log.Errorf("Assembly failed for request %s, couldn't finish request: %s", req.FlowIdentifier, err)
		return file, err
	}

	app.Log.Infof("successfully uploaded fileID %s", file.ID)

	s.cleanupUploadRequest(req.UploadID())

	if alreadyUploaded, uploadedFile := s.uploadedFileInDir(checksum, file.Name, upload.DirectoryID); alreadyUploaded {
		return uploadedFile, nil
	}

	return file, nil
}

// createFile creates the database file entry.
func (s *uploadService) createFile(req *UploadRequest, upload *schema.Upload) (*schema.File, error) {
	file := schema.NewFile(upload.File.Name, upload.ProjectOwner)
	file.Current = false

	f, err := s.files.Insert(&file, upload.DirectoryID, upload.ProjectID)
	app.Log.Infof("Created file %s, in %s %s", f.ID, upload.DirectoryID, upload.ProjectID)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// createDest creates the destination file and ensures that the directory
// path is also created.
func (s *uploadService) createDest(fileID string) error {
	dir := app.MCDir.FileDir(fileID)
	if err := s.fops.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return nil
}

func (s *uploadService) determineChecksum(req *UploadRequest, upload *schema.Upload) string {
	switch {
	case upload.IsExisting:
		// Existing file so use its checksum, no need to compute.
		return upload.File.Checksum
	case upload.ServerRestarted:
		// Server was restarted, so checksum state in tracker is wrong. Read
		// disk file to get the checksum.
		uploadDir := s.requestPath.dir(req.Request)
		hash, _ := file.HashStr(md5.New(), filepath.Join(uploadDir, req.UploadID()))
		return hash
	default:
		// Checksum in tracker is correct since its state has been properly
		// updated as blocks are uploaded.
		return s.tracker.hash(req.UploadID())
	}
}

// cleanup is called when an error has occurred. It attempts to clean up
// the state in the database for this particular entry.
func (s *uploadService) cleanup(req *UploadRequest, fileID string) error {
	upload, err := s.uploads.ByID(req.FlowIdentifier)
	if err != nil {
		return err
	}
	_, err = s.files.Delete(fileID, upload.DirectoryID, upload.ProjectID)
	return err
}

//cleanupUploadRequest removes the upload request and file chunks.
func (s *uploadService) cleanupUploadRequest(uploadID string) {
	s.tracker.clear(uploadID)
	s.uploads.Delete(uploadID)
	s.fops.RemoveAll(app.MCDir.UploadDir(uploadID))
}

func (s *uploadService) uploadedFileInDir(checksum, fileName, dirID string) (bool, *schema.File) {
	files, err := s.dirs.Files(dirID)
	if err != nil {
		return false, nil
	}

	for _, fileEntry := range files {
		if fileEntry.Name == fileName && fileEntry.Checksum == checksum {
			return true, &fileEntry
		}
	}
	return false, nil
}
