package rest

import (
	"github.com/emicklei/go-restful"
	"github.com/materials-commons/mcstore/pkg/ws/rest"
	"github.com/materials-commons/mcstore/server/mcstored/service/rest/upload"
)

// NewServicesContainer creates a new restful.Container made up of all
// the rest resources handled by the server.
func NewServicesContainer() *restful.Container {
	container := restful.NewContainer()
	uploadResource := uploadResource()
	container.Add(uploadResource.WebService())
	return container
}

// uploadResource creates a new upload resource.
func uploadResource() rest.Service {
	tracker := upload.NewUploadTracker()
	finisherFactory := upload.NewUploadFinisherFactory(tracker)
	assemblerFactory := upload.NewMCDirAssemblerFactory(finisherFactory)
	rw := upload.NewFileRequestWriter(upload.NewMCDirRequestPath())
	uploader := upload.NewUploader(rw, tracker)
	return upload.NewResource(uploader, assemblerFactory)
}
