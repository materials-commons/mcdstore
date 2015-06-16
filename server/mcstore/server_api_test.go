package mcstore

import (
	"time"

	"github.com/emicklei/go-restful"
	"github.com/materials-commons/config"
	"github.com/materials-commons/mcstore/pkg/app"
	"github.com/materials-commons/mcstore/pkg/app/flow"
	"github.com/materials-commons/mcstore/pkg/db/dai"
	"github.com/materials-commons/mcstore/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http/httptest"
)

var _ = Describe("ServerApi", func() {
	var (
		api           *ServerAPI
		server        *httptest.Server
		container     *restful.Container
		rr            *httptest.ResponseRecorder
		uploads       dai.Uploads
		uploadRequest CreateUploadRequest
	)

	BeforeEach(func() {
		container = NewServicesContainerForTest()
		server = httptest.NewServer(container)
		rr = httptest.NewRecorder()
		config.Set("mcurl", server.URL)
		config.Set("apikey", "test")
		uploads = dai.NewRUploads(testutil.RSession())
		api = NewServerAPI()
		uploadRequest = CreateUploadRequest{
			ProjectID:     "test",
			DirectoryID:   "test",
			DirectoryPath: "test/test",
			FileName:      "testreq.txt",
			FileSize:      4,
			ChunkSize:     2,
			FileMTime:     time.Now().Format(time.RFC1123),
			Checksum:      "abc123",
		}
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("CreateUploadRequest", func() {
		var resp *CreateUploadResponse
		var err error

		AfterEach(func() {
			if resp != nil {
				uploads.Delete(resp.RequestID)
			}
		})

		It("Should create an upload request", func() {
			resp, err = api.CreateUploadRequest(uploadRequest)
			Expect(err).To(BeNil())
			Expect(resp.RequestID).NotTo(Equal(""))
			Expect(resp.StartingBlock).To(BeNumerically("==", 1))
		})

		It("Should return the same id for a duplicate upload request", func() {
			resp, err = api.CreateUploadRequest(uploadRequest)
			Expect(err).To(BeNil())
			Expect(resp.RequestID).NotTo(Equal(""))
			Expect(resp.StartingBlock).To(BeNumerically("==", 1))

			resp2, err := api.CreateUploadRequest(uploadRequest)
			Expect(err).To(BeNil())
			Expect(resp2.RequestID).To(Equal(resp.RequestID))
			Expect(resp.StartingBlock).To(BeNumerically("==", 1))
		})
	})

	Describe("SendFlowData", func() {
		var flowReq flow.Request
		var resp *CreateUploadResponse
		var err error

		BeforeEach(func() {
			flowReq = flow.Request{
				FlowChunkNumber:  1,
				FlowTotalChunks:  2,
				FlowChunkSize:    2,
				FlowTotalSize:    4,
				FlowFileName:     "testreq.txt",
				FlowRelativePath: "test/testreq.txt",
				ProjectID:        "test",
				DirectoryID:      "test",
			}
		})

		AfterEach(func() {
			if resp != nil {
				uploads.Delete(resp.RequestID)
			}
		})

		It("Should fail on an invalid request id", func() {
			flowReq.FlowIdentifier = "i-dont-exist"
			err = api.SendFlowData(&flowReq)
			Expect(err).To(Equal(app.ErrInvalid))
		})

		It("Should Send the data an increment and increment starting block", func() {
			resp, err = api.CreateUploadRequest(uploadRequest)
			Expect(err).To(BeNil())
			flowReq.FlowIdentifier = resp.RequestID
			err = api.SendFlowData(&flowReq)
			Expect(err).To(BeNil())

			resp2, err := api.CreateUploadRequest(uploadRequest)
			Expect(err).To(BeNil())
			Expect(resp2.RequestID).To(Equal(resp.RequestID))
			Expect(resp2.StartingBlock).To(BeNumerically("==", 2))
		})
	})

	Describe("ListUploadRequests", func() {
		var resp *CreateUploadResponse

		AfterEach(func() {
			if resp != nil {
				uploads.Delete(resp.RequestID)
			}
		})

		It("Should return an empty list when there are no upload requests", func() {
			uploads, err := api.ListUploadRequests("test")
			Expect(err).To(BeNil())
			Expect(uploads).To(HaveLen(0))
		})

		It("Should return a list with one request when a single upload request has been created", func() {
			var err error
			resp, err = api.CreateUploadRequest(uploadRequest)
			Expect(err).To(BeNil())
			uploads, err := api.ListUploadRequests("test")
			Expect(err).To(BeNil())
			Expect(uploads).To(HaveLen(1))
		})
	})

	Describe("DeleteUploadRequest", func() {

	})
})
