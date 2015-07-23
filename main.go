package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/appengine/urlfetch"
	"google.golang.org/cloud"
	"google.golang.org/cloud/storage"
	"io"
	"net/http"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

func init() {
	router := gin.Default()

	fmt.Printf("Running...\n")

	router.POST("/start", startStreamHandler)
	router.POST("/streamPart/:username/:title/:fileName", fileHandler)

	//	router.Run("8080")
	http.Handle("/", router)
}

func startStreamHandler(c *gin.Context) {
	username := c.PostForm("username")
	title := c.PostForm("title")

	c.JSON(200, gin.H{
		"upload_url" : "/streamPart/" + username + "/" + title + "/",
	})
}

type CloudStorageTransport struct {
	next http.RoundTripper
}

func (this *CloudStorageTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Add("x-goog-project-id", "906562518425")
	return this.next.RoundTrip(r)
}

func fileHandler(c *gin.Context) {

	username := c.Param("username")
	title := c.Param("title")
	filename := c.Param("fileName")

	appEngineContext := appengine.NewContext(c.Request)

	hc := &http.Client{
		Transport: &CloudStorageTransport{&oauth2.Transport{
			Source: google.AppEngineTokenSource(appEngineContext, storage.ScopeFullControl),
			Base:   &urlfetch.Transport{Context: appEngineContext},
		}},
	}

	bucketName := "balde_de_bits"
	bucketFile := username + "/" + title + "/"+ filename

	log.Errorf(appEngineContext, "ID ->>> %v", appengine.AppID(appEngineContext))
	log.Errorf(appEngineContext, "File name ->>> %v", bucketFile)

	ctx := cloud.NewContext(appengine.AppID(appEngineContext), hc)
	wc := storage.NewWriter(ctx, bucketName, bucketFile)
	//	wc.ContentType = "image/png"
	defer wc.Close()

	bytesWritten, err := io.Copy(wc, c.Request.Body)

	if err != nil {
		log.Errorf(appEngineContext, "Writing to cloud storage failed. %v", err.Error())
		c.JSON(200, gin.H{
			"response" : "< FAILED >",
		})
		return
	}

	log.Errorf(appEngineContext, "Wrote %v number of bytes, %v", bytesWritten, bucketName)
	log.Errorf(appEngineContext, "File created with name: %v - %v", bucketName, filename)

	c.JSON(200, gin.H{
		"response" : "< worked >",
	})
}
