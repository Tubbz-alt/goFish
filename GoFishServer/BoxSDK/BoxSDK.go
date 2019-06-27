package boxsdk

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
)

// BoxSDK : Reads in JWT and authenticates.
type BoxSDK struct {
	configFile  string
	accessToken string
}

// BoxJWTRequest : Basic structure for a Box API JWT.
type BoxJWTRequest struct {
	BoxAppSettings struct {
		ClientID     string `json:"clientID"`
		ClientSecret string `json:"clientSecret"`
		AppAuth      struct {
			PublicKeyID string `json:"publicKeyID"`
			PrivateKey  string `json:"privateKey"`
			Passphrase  string `json:"passphrase"`
		} `json:"appAuth"`
	} `json:"boxAppSettings"`
	EnterpriseID string `json:"enterpriseID"`
}

type User struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Name  string `json:"name"`
	Login string `json:"login"`
}

// TODO: Fill these out. See "https://developer.box.com/reference" for reference on these.
type FileObject struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	FileVerison struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		SHA1 string `json:"sha1"`
	} `json:"file_version"`
	SequenceID     string `json:"sequence_id"`
	ETag           string `json:"etag"`
	SHA1           string `json:"sha1"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Size           int    `json:"size"`
	PathCollection struct {
		TotalCount int `json:"total_count"`
		Entries    []struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			SequenceID string `json:"sequence_id"`
			ETag       string `json:"etag"`
			Name       string `json:"name"`
		} `json:"entries"`
	} `json:"path_collection"`
	CreatedAt        time.Time `json:"created_at"`
	ModifiedAt       time.Time `json:"modified_at"`
	TrashedAt        time.Time `json:"trashed_at"`
	PurgedAt         time.Time `json:"purged_at"`
	ContentCreatedAt time.Time `json:"content_created_at"`
	ConentModifiedAt time.Time `json:"content_modified_at"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedBy        User      `json:"created_by"`
	ModifiedBy       User      `json:"modified_by"`
	OwnedBy          User      `json:"owned_by"`
	SharedLink       string    `json:"shared_link"`
	Parent           struct {
	} `json:"parent"`
	ItemStatus    string `json:"item_status"`
	VersionNumber string `json:"version_number"`
	CommentCount  int    `json:"comment_count"`
	Permissions   struct {
	} `json:"permissions"`
}

type FolderObject struct {
}

// NewBoxSDK : Creates a new server authenticator.
func NewBoxSDK(file string) *BoxSDK {
	box := &BoxSDK{file, ""}
	os.Setenv("authURL", "https://api.box.com/oauth2/token")
	return box
}

// BoxHTTPRequest : Runs an HTTP request via a defined method.
func (box *BoxSDK) BoxHTTPRequest(method string, url string, payload io.Reader, headers map[string]string) (io.ReadCloser, interface{}, error) {
	client := &http.Client{}

	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		log.Println(err)
		return nil, nil, err
	}

	if headers != nil {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	} else {
		if len(headers) == 0 {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	if box.accessToken != "" {
		req.Header.Add("Authorization", "Bearer "+box.accessToken)
	}

	response, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()

	decoder := json.NewDecoder(response.Body)
	var d interface{}
	decoder.Decode(&d)
	response.Body.Close()

	log.Println(response)
	out, err := os.Create("./downloads/TEST.go")
	if err != nil {
		return nil, nil, err
	}
	defer out.Close()

	_, err = io.Copy(out, response.Body)

	log.Println(" >> Status :", response.StatusCode)
	return response.Body, d, nil
}

// RequestAccessToken : Get valid ACCESS_TOKEN using JWT.
func (box *BoxSDK) RequestAccessToken() {
	name, err := ioutil.ReadFile(box.configFile)
	var boxConfig BoxJWTRequest

	err = json.Unmarshal(name, &boxConfig)

	if err != nil {
		log.Printf("Error: %s\n", err)
	}

	// Create a unique 32 character long string.
	var jti string
	rBytes := make([]byte, 32)
	_, err = rand.Read(rBytes)
	if err == nil {
		jti = base64.URLEncoding.EncodeToString(rBytes)
	}

	// Build the header. This includes the PublicKey as the ID.
	token := jwt.New(jwt.SigningMethodRS512)
	token.Header["keyid"] = boxConfig.BoxAppSettings.AppAuth.PublicKeyID

	// Construct claims.
	claims := token.Claims.(jwt.MapClaims)
	claims["iss"] = boxConfig.BoxAppSettings.ClientID
	claims["sub"] = boxConfig.EnterpriseID
	claims["box_sub_type"] = "enterprise"
	claims["aud"] = os.Getenv("authURL")
	claims["jti"] = jti
	claims["exp"] = time.Now().Add(time.Second * 30).Unix()

	// Decrypt the PrivateKey using its passphrase.
	signedKey, err := jwt.ParseRSAPrivateKeyFromPEMWithPassword(
		[]byte(boxConfig.BoxAppSettings.AppAuth.PrivateKey),
		boxConfig.BoxAppSettings.AppAuth.Passphrase,
	)

	if err != nil {
		log.Println(err)
	}

	// Build the assertion from the signedKey and claims.
	assertion, err := token.SignedString(signedKey)

	if err != nil {
		log.Println(err)
	}

	// Build the access token request.
	payload := url.Values{}
	payload.Add("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	payload.Add("assertion", assertion)
	payload.Add("client_id", boxConfig.BoxAppSettings.ClientID)
	payload.Add("client_secret", boxConfig.BoxAppSettings.ClientSecret)

	// Post the request to the Box API.
	_, response, err := box.BoxHTTPRequest("POST", os.Getenv("authURL"), bytes.NewBufferString(payload.Encode()), nil)
	if err != nil {
		log.Println(err)
		return
	}
	box.accessToken = response.(map[string]interface{})["access_token"].(string)

}

/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// File Functions
/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// UploadFile : Creates an Access Token to the Box API, then uploads a given name to the specified folder.
func (box *BoxSDK) UploadFile(name string, newName string, folderID int) {
	box.RequestAccessToken()

	if newName == "" {
		newName = name
	}

	url := "https://upload.box.com/api/2.0/files/content?attributes={%22name%22:%22" + newName + "%22,%20%22parent%22:{%22id%22:%22" + strconv.Itoa(folderID) + "%22}}"

	payload := strings.NewReader("------WebKitFormBoundary7MA4YWxkTrZu0gW\r\nContent-Disposition: form-data; name=\"file\"; filename=\"" + name + "\"\r\nContent-Type: false\r\n\r\n\r\n------WebKitFormBoundary7MA4YWxkTrZu0gW--")

	headers := make(map[string]string)
	headers["content-type"] = "multipart/form-payload; boundary=----WebKitFormBoundary7MA4YWxkTrZu0gW"

	_, response, err := box.BoxHTTPRequest("POST", url, payload, headers)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(response)
}

// TODO: Figure out a return value for this.
// GetFileInfo : Returns information about the file with 'ID' fileID.
func (box *BoxSDK) GetFileInfo(fileID int) {
	box.RequestAccessToken()
	_, response, err := box.BoxHTTPRequest("GET", "https://api.box.com/2.0/files/"+strconv.Itoa(fileID), nil, nil)
	if err != nil {
		log.Println(err)
		return
	}
	// TODO: Figure out a return value.
	log.Println(response)
}

// TODO: The Box API has a strange way of handling downloads, so need to figure out a work around.
// DownloadFile : Downloads a file with 'ID' fileID.
func (box *BoxSDK) DownloadFile(fileID int) {
	box.RequestAccessToken()
	response, _, err := box.BoxHTTPRequest("GET", "https://api.box.com/2.0/files/"+strconv.Itoa(fileID)+"/content", nil, nil)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(response)
	out, err := os.Create("./downloads/TEST.go")
	if err != nil {
		return
	}
	defer out.Close()

	_, err = io.Copy(out, response)
}

// DeleteFile : Deletes a file in a specific folder with 'ID" fileID.
func (box *BoxSDK) DeleteFile(fileID int, etag string) {
	box.RequestAccessToken()
	headers := make(map[string]string)
	headers["If-Match"] = etag
	_, _, err := box.BoxHTTPRequest("DELETE", "https://api.box.com/2.0/files/"+strconv.Itoa(fileID), nil, headers)
	if err != nil {
		log.Println(err)
		return
	}
}

/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Folder Functions
/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// CreateFolder : Creates a new folder under the parent folder that has 'ID' parentFolderID.
func (box *BoxSDK) CreateFolder(name string, parentFolderID int) {
	box.RequestAccessToken()
	body := strings.NewReader(`{"name":"` + name + `", "parent": {"id": "` + strconv.Itoa(parentFolderID) + `"}}`)

	_, response, err := box.BoxHTTPRequest("POST", "https://api.box.com/2.0/folders", body, nil)
	if err != nil {
		log.Println(err)
		return
	}
	// TODO: Figure out what's happening here.
	log.Println(response)
}

// TODO: Figure out a return value for this.
// GetFolderItems : Returns all the items contained inside the folder with 'ID' folderID.
func (box *BoxSDK) GetFolderItems(folderID int, limit int, offset int) {
	box.RequestAccessToken()

	_, response, err := box.BoxHTTPRequest("GET", "https://api.box.com/2.0/folders/"+strconv.Itoa(folderID)+"/items?limit="+strconv.Itoa(limit)+"&offset="+strconv.Itoa(offset), nil, nil)
	if err != nil {
		log.Println(err)
		return
	}
	// TODO: Figure out what's happening here.
	log.Println(response)
	if response != nil {
		for i, v := range response.(map[string]interface{}) {
			log.Println(i, v)
		}
	}
}

// DeleteFolder : Deletes the folder with 'ID' folderID.
func (box *BoxSDK) DeleteFolder(folderID int) {
	box.RequestAccessToken()
	_, _, err := box.BoxHTTPRequest("DELETE", "https://api.box.com/2.0/folders/"+strconv.Itoa(folderID)+"?recursive=true", nil, nil)
	if err != nil {
		log.Println(err)
		return
	}
}
