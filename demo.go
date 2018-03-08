package wechatbot

import (
	"fmt"
	"log"
	"time"
)

func main() {

	uuid := GetUUID()
	if !GetQrcode(uuid, nil) {
		log.Println("验证码获取失败！")
		return
	}

	for {
		if CheckLogin(uuid) {
			break
		}
		time.Sleep(time.Second)
	}

	log.Println(SKey, Sid, Uin, DeviceID, PassTicket)
	log.Println(NickName, UserName)
	log.Println(SyncKey.String())
	NotifyStatus()
	//登陆成功
	GetAllContact()

	for {
		retcode, selector := SyncCheck()
		log.Println(retcode, selector, SyncKeyToJson(SyncKey))
		if retcode == 0 && selector != 0 {
			b := WebWxSync()
			//log.Println(b.String())

			if b.Get("AddMsgCount").Int() >= 1 {
				a := b.Get("AddMsgList").Array()
				for i := 0; i < len(a); i++ {
					v := a[i]
					if v.Get("MsgType").Int() == 1 {
						fmt.Println(UserNameToNickName[v.Get("FromUserName").String()], UserNameToNickName[v.Get("ToUserName").String()], v.Get("Content").String())
						if v.Get("ToUserName").String() == UserName {
							log.Println()
							time.Sleep(time.Second)
							SendTextMsg(fmt.Sprintf(`镜像反弹：%s`, v.Get("Content").String()), v.Get("FromUserName").String())
							SendFileMsg(`/file/1.jpg`, v.Get("FromUserName").String())
							SendFileMsg(`/file/1.gif`, v.Get("FromUserName").String())
							SendFileMsg(`/file/1.mp4`, v.Get("FromUserName").String())
							SendFileMsg(`/file/1.doc`, v.Get("FromUserName").String())
						}
					}
				}
			}
			if b.Get("Profile").Get("BitFlag").String() == "190" {
				Profile := b.Get("Profile")
				U := Profile.Get("UserName").String()
				N := Profile.Get("NickName").String()
				//添加好友欢迎语
				SendTextMsg(fmt.Sprintf(`你好%s欢迎来到扒拉扒拉`, N), U)
			}

		}
		time.Sleep(1 * time.Second)
	}
}
