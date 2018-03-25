package wechatbot

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"

	"io/ioutil"
	"log"
	"math/rand"
	"mime/multipart"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/h2non/filetype.v1/types"

	"gopkg.in/h2non/filetype.v1"

	httpdo "github.com/546669204/golang-http-do"
	"github.com/mdp/qrterminal"
	"github.com/tidwall/gjson"
	"github.com/tuotoo/qrcode"
)

var SKey, Sid, Uin, DeviceID, PassTicket string
var NickName, UserName string
var SyncKey gjson.Result

var UserNameToNickName map[string]string
var Contact ContactList

var mediaIndex int = 1

type BaseRequest struct {
	Uin      int
	Sid      string
	SKey     string
	DeviceID string
}

type LoginXML struct {
	XMLName     xml.Name `xml:"error"` /* 根节点定义 */
	Ret         string   `xml:"ret"`
	Message     string   `xml:"message"`
	SKey        string   `xml:"skey"`
	WXSid       string   `xml:"wxsid"`
	WXUin       string   `xml:"wxuin"`
	PassTicket  string   `xml:"pass_ticket"`
	IsGrayscale string   `xml:"isgrayscale"`
}
type ContactList struct {
	MemberCount int    `json:"MemberCount"`
	MemberList  []User `json:"MemberList"`
}

type User struct {
	Uin        int64  `json:"Uin"`
	UserName   string `json:"UserName"`
	NickName   string `json:"NickName"`
	RemarkName string `json:"RemarkName"`
	Sex        int8   `json:"Sex"`
	Province   string `json:"Province"`
	City       string `json:"City"`
}
type UserInfoModel struct {
	Name      string
	Pic       string
	LastLogin string
}

var UserInfo UserInfoModel
var SaveFileName string = "wx.data"

func init() {
	httpdo.Autocookieflag = true
}
func JSONMarshal(t interface{}) (string, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)

	return string(buffer.Bytes()), err
}
func GetUUID() (UUID string) {
	UUID = ""
	op := httpdo.Default()
	op.Url = `https://login.wx.qq.com/jslogin?appid=wx782c26e4c19acffb&fun=new&lang=zh_CN&_=` + fmt.Sprintf(`%s`, time.Now().Unix())
	httpbyte, err := httpdo.HttpDo(op)
	if err != nil {
		log.Println(err)
		return
	}
	reg := regexp.MustCompile(`^window.QRLogin.code = (\d+); window.QRLogin.uuid = "(\S+)";$`)
	matches := reg.FindStringSubmatch(string(httpbyte))
	if len(matches) != 3 {
		log.Println("解析数据失败" + string(httpbyte))
		return
	}
	status, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		log.Println(err)
		return
	}
	if status != 200 {
		log.Println("返回状态码错误")
		return
	}
	UUID = matches[2]
	return
}

func GetQrcode(UUID string, QrcodeStr *string) bool {
	op := httpdo.Default()
	op.Url = `https://login.weixin.qq.com/qrcode/` + UUID
	httpbyte, err := httpdo.HttpDo(op)
	if err != nil {
		log.Println(err)
		return false
	}
	M, err := qrcode.Decode(bytes.NewReader(httpbyte))
	if QrcodeStr == nil {
		qrterminal.GenerateHalfBlock(M.Content, qrterminal.L, os.Stdout)
	} else {
		*QrcodeStr = M.Content
	}

	return true
}
func GetRandomString(index int, length int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bytes := []byte(str)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < length; i++ {
		result = append(result, bytes[r.Intn(index)])
	}
	return string(result)
}

func CheckLogin(UUID string) bool {
	op := httpdo.Default()
	var timestamp int64 = time.Now().UnixNano() / 1000000
	op.Url = fmt.Sprintf("https://login.wx.qq.com/cgi-bin/mmwebwx-bin/login?loginicon=true&uuid=%s&tip=0&r=%d&_=%d", UUID, ^(int32)(timestamp), timestamp)
	httpbyte, err := httpdo.HttpDo(op)
	if err != nil {
		log.Println(err)
		return false
	}

	reg := regexp.MustCompile(`^window.code=(\d+);`)
	matches := reg.FindStringSubmatch(string(httpbyte))
	if len(matches) < 2 {
		log.Println("返回格式不正确")
		return false
	}

	status, _ := strconv.Atoi(matches[1])

	switch status {
	case 200:
		reg := regexp.MustCompile(`window.redirect_uri="(\S+)";`)
		matches := reg.FindStringSubmatch(string(httpbyte))
		if len(matches) < 2 {
			log.Println("重写url错误")
			return false
		}
		op = httpdo.Default()
		op.Url = matches[1] + "&fun=new&version=v2"
		httpbyte, err = httpdo.HttpDo(op)
		if err != nil {
			log.Println(err)
			return false
		}
		//log.Println(string(httpbyte))
		loginXML := LoginXML{}
		err = xml.Unmarshal(httpbyte, &loginXML)

		SKey = loginXML.SKey
		Sid = loginXML.WXSid
		Uin = loginXML.WXUin
		PassTicket = loginXML.PassTicket
		DeviceID = "e" + GetRandomString(10, 15)

		op = httpdo.Default()
		op.Method = "POST"
		op.Url = fmt.Sprintf("https://wx.qq.com/cgi-bin/mmwebwx-bin/webwxinit?lang=zh_CN&r=%d&pass_ticket=%s", time.Now().Unix(), PassTicket)
		op.Data = fmt.Sprintf(`{"BaseRequest":{"Uin":"%s","Sid":"%s","Skey":"%s","DeviceID":"%s"}}`, Uin, Sid, SKey, DeviceID)
		op.Header = "Content-Type:application/json;charset=UTF-8"

		httpbyte, err = httpdo.HttpDo(op)
		if err != nil {
			log.Println(err)
			return false
		}
		//log.Println(string(httpbyte))
		user := gjson.Get(string(httpbyte), "User")
		SyncKey = gjson.Get(string(httpbyte), "SyncKey")
		UserName = user.Get("UserName").String()
		NickName = user.Get("NickName").String()
		UserInfo.Name = NickName
		UserInfo.LastLogin = time.Now().Format("2006-01-02 15:04:05")
		return true
		break
	case 201:
		log.Println("请在手机上确认")
		reg := regexp.MustCompile(`window.userAvatar = '(\S+)';`)
		matches := reg.FindStringSubmatch(string(httpbyte))
		if len(matches) == 2 {
			UserInfo.Pic = matches[1]
		}
		return false
		break
	case 408:
		log.Println("请扫描二维码")
		return false
		break
	default:
		log.Println(status)
		return false
		break
	}
	return false

}
func NotifyStatus() bool {
	op := httpdo.Default()
	op.Method = "POST"
	op.Url = fmt.Sprintf("https://wx.qq.com/cgi-bin/mmwebwx-bin/webwxstatusnotify?pass_ticket=%s", PassTicket)
	op.Data =
		fmt.Sprintf(`{"BaseRequest":{"Uin":%s,"Sid":"%s","Skey":"%s","DeviceID":"%s"},"Code":3,"FromUserName":"%s","ToUserName":"%s","ClientMsgId":%d}`, Uin, Sid, SKey, DeviceID, UserName, UserName, time.Now().UnixNano()/1000000)
	op.Header = "Content-Type:application/json;charset=UTF-8"
	_, err := httpdo.HttpDo(op)
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}
func GetAllContact() string {
	op := httpdo.Default()
	op.Url = fmt.Sprintf("https://wx.qq.com/cgi-bin/mmwebwx-bin/webwxgetcontact?lang=zh_CN&r=%d&pass_ticket=%s&seq=0&skey=%s", time.Now().Unix(), PassTicket, SKey)
	httpbyte, err := httpdo.HttpDo(op)
	if err != nil {
		log.Println(err)
		return ""
	}

	log.Printf("一共%d位联系人\n", gjson.Get(string(httpbyte), "MemberCount").Int())
	json.Unmarshal(httpbyte, &Contact)
	MemberList := gjson.Get(string(httpbyte), "MemberList").Array()
	UserNameToNickName = make(map[string]string)
	for index := 0; index < len(MemberList); index++ {
		v := MemberList[index]
		UserNameToNickName[v.Get("UserName").String()] = v.Get("NickName").String()
		log.Printf("%s=======%s", v.Get("NickName").String(), v.Get("UserName").String())
	}
	return string(httpbyte)
}
func SyncKeyToString(S gjson.Result) string {
	//log.Println(S.String())
	var rs []string
	array := S.Get("List").Array()
	for index := 0; index < len(array); index++ {
		v := array[index]
		rs = append(rs, fmt.Sprintf("%d_%d", v.Get("Key").Int(), v.Get("Val").Int()))
	}
	return strings.Join(rs, "|")
}
func SyncKeyToJson(S gjson.Result) string {
	str := S.String()
	str = strings.Replace(str, " ", "", -1)
	// 去除换行符
	str = strings.Replace(str, "\n", "", -1)
	return str
}
func SyncCheck() (retcode, selector int) {
	TimeStamp := time.Now().UnixNano() / 1000000
	op := httpdo.Default()
	op.Url = fmt.Sprintf("https://webpush.wx.qq.com/cgi-bin/mmwebwx-bin/synccheck?r=%d&skey=%s&sid=%s&uin=%s&deviceid=%s&synckey=%s&_=%d", TimeStamp, SKey, Sid, Uin, DeviceID, SyncKeyToString(SyncKey), TimeStamp)
	//op.Url = strings.Replace(op.Url, "|", "%7C", -1)
	//op.Url = strings.Replace(op.Url, "@", "%40", -1)
	//log.Println(op.Url)
	httpbyte, err := httpdo.HttpDo(op)
	if err != nil {
		log.Println(err)
		return
	}
	/* 根据正则得到selector => window.synccheck={retcode:"0",selector:"0"}*/
	reg := regexp.MustCompile(`^window.synccheck={retcode:"(\d+)",selector:"(\d+)"}$`)
	matches := reg.FindStringSubmatch(string(httpbyte))

	retcode, err = strconv.Atoi(matches[1])
	if err != nil {
		log.Println("数据有问题")
		return
	}

	selector, err = strconv.Atoi(matches[2])
	if err != nil {
		log.Println("数据有问题")
		return
	}

	if retcode == 1101 {
		log.Println("帐号已在其他地方登陆")
		return
	}
	if retcode == 1102 {
		log.Println("移动端退出")
		return
	}
	if retcode != 0 {
		log.Println("未知返回码")
		return
	}
	return
}

func WebWxSync() gjson.Result {
	op := httpdo.Default()
	op.Method = "POST"
	op.Url = fmt.Sprintf("https://wx.qq.com/cgi-bin/mmwebwx-bin/webwxsync?sid=%s&skey=%s&pass_ticket=%s", Sid, SKey, PassTicket)
	op.Data =
		fmt.Sprintf(`{"BaseRequest":{"Uin":%s,"Sid":"%s","Skey":"%s","DeviceID":"%s"},"SyncKey":%s,"rr":%d}`, Uin, Sid, SKey, DeviceID, SyncKeyToJson(SyncKey), -time.Now().Unix())
	op.Header = "Content-Type:application/json;charset=UTF-8"
	httpbyte, err := httpdo.HttpDo(op)
	if err != nil {
		log.Println(err)
		return gjson.Result{}
	}
	SyncKey = gjson.Get(string(httpbyte), "SyncKey")
	//log.Println(string(httpbyte))

	return gjson.ParseBytes(httpbyte)
}

func SendMsg(con map[string]interface{}) {
	op := httpdo.Default()
	op.Method = "POST"
	op.Url = fmt.Sprintf("https://wx.qq.com/cgi-bin/mmwebwx-bin/webwxsendmsg?lang=zh_CN&pass_ticket=%s", PassTicket)
	jsoncon, err := JSONMarshal(con)
	if err != nil {
		return
	}
	op.Data =
		fmt.Sprintf(`{"BaseRequest":{"Uin":%s,"Sid":"%s","Skey":"%s","DeviceID":"%s"},"Msg":%s,"Scene":0}`, Uin, Sid, SKey, DeviceID, jsoncon)
	op.Header = "Content-Type:application/json;charset=UTF-8"
	_, err = httpdo.HttpDo(op)
	if err != nil {
		log.Println(err)
		return
	}
}
func SendMsgImage(con map[string]interface{}) {
	op := httpdo.Default()
	op.Method = "POST"
	op.Url = fmt.Sprintf("https://wx.qq.com/cgi-bin/mmwebwx-bin/webwxsendmsgimg?fun=async&f=json&pass_ticket=%s", PassTicket)
	jsoncon, err := JSONMarshal(con)
	if err != nil {
		return
	}
	op.Data =
		fmt.Sprintf(`{"BaseRequest":{"Uin":%s,"Sid":"%s","Skey":"%s","DeviceID":"%s"},"Msg":%s,"Scene":0}`, Uin, Sid, SKey, DeviceID, jsoncon)
	op.Header = "Content-Type:application/json;charset=UTF-8"
	_, err = httpdo.HttpDo(op)
	if err != nil {
		log.Println(err)
		return
	}
}
func SendMsgVideo(con map[string]interface{}) {
	op := httpdo.Default()
	op.Method = "POST"
	op.Url = fmt.Sprintf("https://wx.qq.com/cgi-bin/mmwebwx-bin/webwxsendvideomsg?fun=async&f=json&pass_ticket=%s", PassTicket)
	jsoncon, err := JSONMarshal(con)
	if err != nil {
		return
	}
	op.Data =
		fmt.Sprintf(`{"BaseRequest":{"Uin":%s,"Sid":"%s","Skey":"%s","DeviceID":"%s"},"Msg":%s,"Scene":0}`, Uin, Sid, SKey, DeviceID, jsoncon)
	op.Header = "Content-Type:application/json;charset=UTF-8"
	_, err = httpdo.HttpDo(op)
	if err != nil {
		log.Println(err)
		return
	}
}
func SendMsgFile(con map[string]interface{}) {
	op := httpdo.Default()
	op.Method = "POST"
	op.Url = fmt.Sprintf("https://wx.qq.com/cgi-bin/mmwebwx-bin/webwxsendappmsg?fun=async&f=json&pass_ticket=%s", PassTicket)
	jsoncon, err := JSONMarshal(con)
	if err != nil {
		return
	}
	op.Data =
		fmt.Sprintf(`{"BaseRequest":{"Uin":%s,"Sid":"%s","Skey":"%s","DeviceID":"%s"},"Msg":%s,"Scene":0}`, Uin, Sid, SKey, DeviceID, jsoncon)
	op.Header = "Content-Type:application/json;charset=UTF-8"
	_, err = httpdo.HttpDo(op)
	if err != nil {
		log.Println(err)
		return
	}
}
func SendMsgEmoticon(con map[string]interface{}) {
	op := httpdo.Default()
	op.Method = "POST"
	op.Url = fmt.Sprintf("https://wx.qq.com/cgi-bin/mmwebwx-bin/webwxsendemoticon?fun=sys&f=json&pass_ticket=%s", PassTicket)
	jsoncon, err := JSONMarshal(con)
	if err != nil {
		return
	}
	op.Data =
		fmt.Sprintf(`{"BaseRequest":{"Uin":%s,"Sid":"%s","Skey":"%s","DeviceID":"%s"},"Msg":%s,"Scene":0}`, Uin, Sid, SKey, DeviceID, jsoncon)
	op.Header = "Content-Type:application/json;charset=UTF-8"
	_, err = httpdo.HttpDo(op)
	if err != nil {
		log.Println(err)
		return
	}
}
func InviteMember(memberUserName string, chatRoomUserName string) {
	op := httpdo.Default()
	op.Method = "POST"
	op.Url = fmt.Sprintf("https://wx.qq.com/cgi-bin/mmwebwx-bin/webwxupdatechatroom?fun=invitemember")
	op.Data =
		fmt.Sprintf(`{"BaseRequest":{"Uin":%s,"Sid":"%s","Skey":"%s","DeviceID":"%s"},"InviteMemberList":"%s","ChatRoomName":"%s"}`, Uin, Sid, SKey, DeviceID, memberUserName, chatRoomUserName)
	op.Header = "Content-Type:application/json;charset=UTF-8"
	_, err := httpdo.HttpDo(op)
	if err != nil {
		log.Println(err)
		return
	}
}
func SendTextMsg(content, to string) error {
	var msg = make(map[string]interface{})
	msg["FromUserName"] = UserName
	msg["ToUserName"] = to
	msg["Content"] = content
	msg["Type"] = 1
	msg["LocalID"] = fmt.Sprintf("%d", time.Now().Unix())
	msg["ClientMsgId"] = msg["LocalID"]
	SendMsg(msg)
	return nil
}

func SendFileMsg(filepath, to string) error {

	info, err := os.Stat(filepath)
	if err != nil {
		return err
	}

	buf, err := ioutil.ReadFile(filepath)
	if err != nil {
		return err
	}
	kind, _ := filetype.Get(buf)

	media, err := UploadMedia(buf, kind, info, to)

	if err != nil {
		return err
	}
	var msg = make(map[string]interface{})
	msg["FromUserName"] = UserName
	msg["ToUserName"] = to
	msg["LocalID"] = fmt.Sprintf("%d", time.Now().Unix())
	msg["ClientMsgId"] = msg["LocalID"]

	if filetype.IsImage(buf) {
		if strings.HasSuffix(kind.MIME.Value, `gif`) {
			msg["Type"] = 47
			msg["MediaId"] = media
			msg["EmojiFlag"] = 2
			SendMsgEmoticon(msg)
		} else {
			msg["Type"] = 3
			msg["MediaId"] = media
			SendMsgImage(msg)
		}
	} else {
		info, _ := os.Stat(filepath)
		if filetype.IsVideo(buf) {
			msg["Type"] = 43
			msg["MediaId"] = media
			SendMsgVideo(msg)
		} else {
			msg["Type"] = 6
			msg[`Content`] = fmt.Sprintf(`<appmsg appid='wxeb7ec651dd0aefa9' sdkver=''><title>%s</title><des></des><action></action><type>6</type><content></content><url></url><lowurl></lowurl><appattach><totallen>10</totallen><attachid>%s</attachid><fileext>%s</fileext></appattach><extinfo></extinfo></appmsg>`, info.Name(), media, kind.Extension)
			SendMsgFile(msg)
		}
	}

	return err
}
func UploadMedia(buf []byte, kind types.Type, info os.FileInfo, to string) (string, error) {

	// Only the first 261 bytes are used to sniff the content type.
	head := buf[:261]

	var mediatype string
	if filetype.IsImage(head) {
		mediatype = `pic`
	} else if filetype.IsVideo(head) {
		mediatype = `video`
	} else {
		mediatype = `doc`
	}

	fields := map[string]string{
		`id`:                `WU_FILE_` + fmt.Sprintf("%d", mediaIndex),
		`name`:              info.Name(),
		`type`:              kind.MIME.Value,
		`lastModifiedDate`:  info.ModTime().UTC().String(),
		`size`:              fmt.Sprintf("%d", info.Size()),
		`mediatype`:         mediatype,
		`pass_ticket`:       PassTicket,
		`webwx_data_ticket`: CookieDataTicket(),
	}
	md5Ctx := md5.New()
	md5Ctx.Write(buf)
	cipherStr := md5Ctx.Sum(nil)
	media, err := json.Marshal(&map[string]interface{}{
		`BaseRequest`:   GetBaseRequestStr(),
		`ClientMediaId`: fmt.Sprintf("%d", time.Now().Unix()),
		`TotalLen`:      fmt.Sprintf("%d", info.Size()),
		`StartPos`:      0,
		`DataLen`:       fmt.Sprintf("%d", info.Size()),
		`MediaType`:     4,
		`UploadType`:    2,
		`ToUserName`:    to,
		`FromUserName`:  UserName,
		`FileMd5`:       hex.EncodeToString(cipherStr),
	})

	if err != nil {
		return ``, err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fw, err := writer.CreateFormFile(`filename`, info.Name())
	if err != nil {
		return ``, err
	}
	fw.Write(buf)

	for k, v := range fields {
		writer.WriteField(k, v)
	}

	writer.WriteField(`uploadmediarequest`, string(media))
	writer.Close()
	postdata, _ := ioutil.ReadAll(body)
	op := httpdo.Default()
	op.Method = "POST"
	/*for _, k := range []string{"", "2"} {
		op.Url = `https://file` + k + `.wx.qq.com/cgi-bin/mmwebwx-bin/webwxuploadmedia?f=json`
	}*/
	op.Url = `https://file.wx.qq.com/cgi-bin/mmwebwx-bin/webwxuploadmedia?f=json`
	op.Data = postdata
	op.Header = `Content-Type:` + writer.FormDataContentType()
	httpbyte, err := httpdo.HttpDo(op)
	if err != nil {
		return ``, err
	}
	mediaIndex++

	return gjson.Get(string(httpbyte), "MediaId").String(), nil
}

func CookieDataTicket() string {
	urlPath, err := url.Parse("https://wx.qq.com/")
	if err != nil {
		return ``
	}
	ticket := ``
	cookies := httpdo.Autocookie.Cookies(urlPath)
	for _, cookie := range cookies {
		if cookie.Name == `webwx_data_ticket` {
			ticket = cookie.Value
			break
		}
	}

	return ticket
}

func GetBaseRequestStr() BaseRequest {
	var s BaseRequest
	ui, _ := strconv.Atoi(Uin)
	s.Uin = ui
	s.Sid = Sid
	s.SKey = SKey
	s.DeviceID = DeviceID
	return s
}

func SaveLogin() {
	httpdo.SaveCookies()
	file, _ := os.OpenFile(SaveFileName, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0)
	defer file.Close()
	file.Write([]byte(fmt.Sprintf(`{"Uin":"%s","Sid":"%s","Skey":"%s","DeviceID":"%s","PassTicket":"%s","NickName":"%s","UserName":"%s","SyncKey":%s,"mediaIndex":"%d","pic":"%s"}`, Uin, Sid, SKey, DeviceID, PassTicket, NickName, UserName, strings.Replace(SyncKey.Raw, "\n", "", -1), mediaIndex, UserInfo.Pic)))
	return
}
func LoadLogin() bool {
	httpdo.LoadCookies()
	file, err := os.OpenFile(SaveFileName, os.O_RDWR, 0)
	if os.IsNotExist(err) {
		return false
	}
	var str string
	var strbyte = make([]byte, 1024)
	for {
		n, err := file.Read(strbyte)
		if err != nil && err != io.EOF {
			log.Println(err)
		}
		if 0 == n {
			break
		}
		str = str + string(strbyte[:n])
	}

	if !gjson.Valid(str) {
		return false
	}
	data := gjson.Parse(str)
	Uin = data.Get("Uin").String()
	Sid = data.Get("Sid").String()
	SKey = data.Get("SKey").String()
	DeviceID = data.Get("DeviceID").String()
	PassTicket = data.Get("PassTicket").String()
	NickName = data.Get("NickName").String()
	UserName = data.Get("UserName").String()
	SyncKey = data.Get("SyncKey")
	mediaIndex = int(data.Get("mediaIndex").Int())
	UserInfo.Name = NickName
	UserInfo.LastLogin = time.Now().Format("2006-01-02 15:04:05")
	UserInfo.Pic = data.Get("pic").String()
	return true
}
func IsLogin() bool {
	retcode, _ := SyncCheck()
	if retcode != 0 {
		return false
	}
	return true
}

func SetRemark(remark, to string) (bool, error) {
	op := httpdo.Default()
	op.Url = "https://wx.qq.com/cgi-bin/mmwebwx-bin/webwxoplog"
	op.Method = "POST"
	op.Data = fmt.Sprintf(`{"UserName":"%s","CmdId":2,"RemarkName":"%s","BaseRequest":{"Uin":"%s","Sid":"%s","Skey":"%s","DeviceID":"%s"}}`, to, remark, Uin, Sid, SKey, DeviceID)
	httpbyte, err := httpdo.HttpDo(op)

	if gjson.ParseBytes(httpbyte).Get("Ret").Int() == 0 {
		return true, err
	}
	return false, err
}
