package main

import (
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
	"strings"
)

func init() {
	router := gin.Default()

	router.POST("/start", startStreamHandler)
	router.POST("/streamPart/:username/:title/:fileName", fileHandler)
	router.GET("/list", listHandler)

	//	router.Run("8080")
	http.Handle("/", router)
}

func startStreamHandler(c *gin.Context) {
	username := c.PostForm("username")
	title := c.PostForm("title")

	c.JSON(200, gin.H{
		"upload_url" : "/streamPart/" + username + "/" + title + "/",
		"stream_id" :  "stream-" + username + "-" + title,
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

	gaeContext := appengine.NewContext(c.Request)

	hc := &http.Client{
		Transport: &CloudStorageTransport{&oauth2.Transport{
			Source: google.AppEngineTokenSource(gaeContext, storage.ScopeFullControl),
			Base:   &urlfetch.Transport{Context: gaeContext},
		}},
	}

	bucketName := "balde_de_bits"
	bucketFile := username + "/" + title + "/"+ filename

	log.Errorf(gaeContext, "ID ->>> %v", appengine.AppID(gaeContext))
	log.Errorf(gaeContext, "File name ->>> %v", bucketFile)

	ctx := cloud.NewContext(appengine.AppID(gaeContext), hc)
	wc := storage.NewWriter(ctx, bucketName, bucketFile)


	if strings.Contains(filename, "m3u8") {
		wc.ContentType = "application/x-mpegURL"
		wc.CacheControl = "max-age:0"
	} else if strings.Contains(filename, "ts") {
		wc.ContentType = "video/MP2T"
	} else if strings.Contains(filename, "jpg") {
		wc.ContentType = "image/jpeg"
	}

	defer wc.Close()

	bytesWritten, err := io.Copy(wc, c.Request.Body)

	if err != nil {
		log.Errorf(gaeContext, "Writing to cloud storage failed. %v", err.Error())
		c.JSON(200, gin.H{
			"response" : "< FAILED >",
		})
		return
	}

	log.Errorf(gaeContext, "Wrote < %v > bytes for file < %v >", bytesWritten, filename)

	c.JSON(200, gin.H{
		"response" : "< worked >",
	})
}

type Stream struct {
	Title        string `json:"title"`
	IndexUrl     string `json:"index_url"`
	VodUrl       string `json:"vod_url"`
	ThumbnailUrl string `json:"thumbnail_url"`

}

type User struct {
	Name    string `json:"name"`
	Streams []Stream `json:"streams"`
}

func listHandler(c *gin.Context) {
	gaeContext := appengine.NewContext(c.Request)

	fhc := &http.Client{
		Transport: &CloudStorageTransport{&oauth2.Transport{
			Source: google.AppEngineTokenSource(gaeContext, storage.ScopeFullControl),
			Base:   &urlfetch.Transport{Context: gaeContext},
		}},
	}

	bucketName := "balde_de_bits"

	cloudContext := cloud.NewContext(appengine.AppID(gaeContext), fhc)

	objects, _ := storage.ListObjects(cloudContext, bucketName, nil)

	usersMap := mapFilesToDictionary(objects)
	usersStruct := mapDictionaryToObjects(usersMap)


	c.JSON(200, gin.H{
		"users": usersStruct,
	})
}

func mapFilesToDictionary(objects *storage.Objects) (users map[string]map[string]map[string]string) {

	users = map[string]map[string]map[string]string{}

	for _, result := range objects.Results {
		if !(strings.Contains(result.Name, ".m3u8") || (strings.Contains(result.Name, ".jpg"))) {
			continue
		}
		userString, titleString, fileString := extractValues(result)

		if _, ok := users[userString]; !ok {
			users[userString] = map[string]map[string]string{}
		}

		titles := users[userString]
		if _, ok := titles[titleString]; !ok {
			titles[titleString] = map[string]string{}
		}

		base := "https://storage.googleapis.com/" + result.Bucket + "/" + result.Name
		if strings.Contains(fileString, "vod") {
			titles[titleString]["vod_url"] = base
		} else if strings.Contains(fileString, "index") {
			titles[titleString]["index_url"] = base
		} else {
			titles[titleString]["thumbnail_url"] = base
		}
	}

	return users
}

func mapDictionaryToObjects(usersMap map[string]map[string]map[string]string) (users []User) {
	users = []User{}
	for username, userMap := range usersMap {
		user := User{Name:username, Streams:[]Stream{}}
		for titleName, titleMap := range userMap {
			title := Stream{Title: titleName}
			for typeName, url := range titleMap {
				if strings.Contains(typeName, "vod") {
					title.VodUrl = url
				} else if strings.Contains(typeName, "index") {
					title.IndexUrl = url
				} else {
					title.ThumbnailUrl = url
				}
			}

			user.Streams = append(user.Streams, title)
		}

		users = append(users, user)
	}

	return users
}

func extractValues(result *storage.Object) (user string, title string, file string) {
	slices := strings.Split(result.Name, "/")
	return slices[0], slices[1], slices[2]
}
