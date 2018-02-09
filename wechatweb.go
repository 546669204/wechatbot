package wechatbot

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

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
type WxSendMsg struct {
	Type         int    `json:"Type"`
	Content      string `json:"Content"`
	FromUserName string `json:"FromUserName"`
	ToUserName   string `json:"ToUserName"`
	LocalID      string `json:"LocalID"`
	ClientMsgId  string `json:"ClientMsgId"`
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

func GetQrcode(UUID string) bool {
	op := httpdo.Default()
	op.Url = `https://login.weixin.qq.com/qrcode/` + UUID
	httpbyte, err := httpdo.HttpDo(op)
	if err != nil {
		log.Println(err)
		return false
	}
	M, err := qrcode.Decode(bytes.NewReader(httpbyte))

	qrterminal.GenerateHalfBlock(M.Content, qrterminal.L, os.Stdout)
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
		return true
		break
	case 201:
		log.Println("请在手机上确认")
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
func GetAllContact() {
	op := httpdo.Default()
	op.Url = fmt.Sprintf("https://wx.qq.com/cgi-bin/mmwebwx-bin/webwxgetcontact?lang=zh_CN&r=%d&pass_ticket=%s&seq=0&skey=%s", time.Now().Unix(), PassTicket, SKey)
	httpbyte, err := httpdo.HttpDo(op)
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("一共%s位联系人\n", gjson.Get(string(httpbyte), "MemberCount").Int())
	json.Unmarshal(httpbyte, &Contact)
	MemberList := gjson.Get(string(httpbyte), "MemberList").Array()
	UserNameToNickName = make(map[string]string)
	for index := 0; index < len(MemberList); index++ {
		v := MemberList[index]
		UserNameToNickName[v.Get("UserName").String()] = v.Get("NickName").String()
		log.Printf("%s=======%s", v.Get("NickName").String(), v.Get("UserName").String())
	}

}
func SyncKeyToString(S gjson.Result) string {
	//log.Println(S.String())
	var rs string = ""
	array := S.Get("List").Array()
	for index := 0; index < len(array); index++ {
		v := array[index]
		rs += fmt.Sprintf("%d_%d|", v.Get("Key").Int(), v.Get("Val").Int())
	}
	return rs[:len(rs)-1]
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
	log.Println(string(httpbyte))

	return gjson.ParseBytes(httpbyte)
}

func SendMsg(con WxSendMsg) {
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
	//log.Println(string(httpbyte))
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
