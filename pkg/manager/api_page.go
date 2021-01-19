package manager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Klevry/klevr/pkg/common"
	"github.com/NexClipper/logger"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type PageAPI struct{}

// InitPage initialize page API
// @title Klevr-Dashboard API
// @version 1.0
// @description
// @contact.name name
// @contact.email email@email.com
// @BasePath /
func (api *API) InitPage(page *mux.Router) {
	logger.Debug("API InitPage - init URI")

	// 사용자는 admin만 존재하는 가정
	tx := &Tx{api.DB.NewSession()}
	cnt, _ := tx.getPageMember("admin")
	if cnt == 0 {
		encPassword, err := common.Encrypt(api.Manager.Config.Server.EncryptionKey, "admin")
		if err == nil {
			apiKey := strings.Replace(uuid.New().String(), "-", "", -1)
			p := &PageMembers{
				UserId:       "admin",
				UserPassword: encPassword,
				Activated:    false,
				ApiKey:       apiKey,
			}
			tx.insertPageMember(p)
		} else {
			logger.Error(err)
		}
	}

	pageAPI := &PageAPI{}

	registURI(page, POST, "/signin", pageAPI.SignIn)
	registURI(page, GET, "/signout", pageAPI.SignOut)
	registURI(page, POST, "/changepassword", pageAPI.ChangePassword)
	registURI(page, GET, "/activated/{id}", pageAPI.Activated)
}

// SignIn API
// @Summary 사용자 인증을 한다.
// @Description Klevr 대시보드를 사용하기 위한 인증을 제공한다.
// @Tags Page
// @Accept mpfd
// @Router /page/signin [post]
// @Param id "admin"
// @Param pw 사용자 패스워드
// @Success 200
func (api *PageAPI) SignIn(w http.ResponseWriter, r *http.Request) {
	ctx := CtxGetFromRequest(r)
	tx := GetDBConn(ctx)

	manager := CtxGetServer(ctx)

	id := r.FormValue("id")
	pw := r.FormValue("pw")

	if id != "admin" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	cnt, pms := tx.getPageMember(id)
	if cnt == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	pm := (*pms)[0]

	if pm.Activated == false {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	decPassword, err := common.Decrypt(manager.Config.Server.EncryptionKey, pm.UserPassword)
	if err != nil || pw != decPassword {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	resp, err := json.Marshal(struct {
		Status string `json:"apikey"`
	}{
		pm.ApiKey,
	})
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	expirationTime := time.Now().Add(1 * time.Hour)
	jwtHelper := common.NewJWTHelper([]byte(manager.Config.Page.Secret)).AddClaims("id", id).SetExpirationTime(expirationTime.Unix())
	tks, err := jwtHelper.GenToken()
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	http.SetCookie(w, &http.Cookie{Name: "token", Value: tks, Expires: expirationTime})
	w.WriteHeader(200)
	fmt.Fprintf(w, "%s", resp)
}

func (api *PageAPI) SignOut(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Name:    "token",
		Value:   "",
		Expires: time.Now(),
		MaxAge:  -1,
	}

	http.SetCookie(w, cookie)
	w.WriteHeader(200)
}

func (api *PageAPI) ChangePassword(w http.ResponseWriter, r *http.Request) {
	ctx := CtxGetFromRequest(r)
	tx := GetDBConn(ctx)

	manager := CtxGetServer(ctx)

	id := r.FormValue("id")
	pw := r.FormValue("pw")
	cpw := r.FormValue("cpw") // confirmed password

	if id != "admin" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	cnt, pms := tx.getPageMember(id)
	if cnt == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	pm := (*pms)[0]
	if pm.Activated == true {
		decPassword, err := common.Decrypt(manager.Config.Server.EncryptionKey, pm.UserPassword)
		if err != nil || pw != decPassword {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	encPassword, err := common.Encrypt(manager.Config.Server.EncryptionKey, cpw)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	pm.UserPassword = encPassword
	pm.Activated = true
	tx.updatePageMember(&pm)

	w.WriteHeader(200)
}

func (api *PageAPI) Activated(w http.ResponseWriter, r *http.Request) {
	ctx := CtxGetFromRequest(r)
	tx := GetDBConn(ctx)

	vars := mux.Vars(r)
	userID := vars["id"]

	cnt, pms := tx.getPageMember(userID)
	if cnt == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	pm := (*pms)[0]
	var activatedStatus string
	if pm.Activated == true {
		activatedStatus = "activated"
	} else {
		activatedStatus = "initialized"
	}

	resp, err := json.Marshal(struct {
		Status string `json:"status"`
	}{
		activatedStatus,
	})
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	w.WriteHeader(200)
	fmt.Fprintf(w, "%s", resp)
}
