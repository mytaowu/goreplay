// Package message 可用于相关信息提醒的包。
package message

import (
	"fmt"
	"net"
	"strings"
	"time"

	"goreplay/logger"
	"goreplay/logreplay"
)

// ExitRobotMsg 退出时的微信机器人通知消息。
type ExitRobotMsg struct {
	Module logreplay.Module
	Time   time.Duration
}

// ReadMsg create a msg
func (msg *ExitRobotMsg) ReadMsg() string {
	owners := strings.Join(msg.Module.Owners, ",")
	ipAddress := msg.getLocalIPAdd()

	return fmt.Sprintf(
		`goreplay后台录制提醒 %s服务录制详情，请相关同事注意。
			   app_id:<font color="comment">%s</font>
			   module_id:<font color="comment">%s</font>
			   module_name_en:<font color="comment">%s</font>
			   app_name_en:<font color="comment">%s</font>
			   module_name_ch:<font color="comment">%s</font>
			   IP address:<font color="comment">%s</font>
			   owners:<font color="comment">%s</font>
			   goreplay 服务通知，AppName:%s, ModuleName:%s已经录制了%s
			   如不需要进一步录制，请前往相应容器关闭！`,
		msg.Module.AppNameEn, msg.Module.AppID, msg.Module.ModuleID,
		msg.Module.ModuleNameEn, msg.Module.AppNameEn, msg.Module.ModuleNameCh,
		ipAddress, owners, msg.Module.AppNameEn, msg.Module.ModuleNameEn, msg.Time)
}

func (msg *ExitRobotMsg) getLocalIPAdd() string {
	addressList, err := net.InterfaceAddrs()
	if err != nil {
		logger.Debug2("get container address err :", err)
		return "未知"
	}

	strAdds := make([]string, 0)
	for _, address := range addressList {
		// 检查ip地址判断是否回环地址
		if ipNet, ok := address.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				strAdds = append(strAdds, ipNet.IP.String())
			}
		}
	}

	return strings.Join(strAdds, ",")
}
