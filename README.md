# gsip
 基于 Golang 实现的 SIP 协议栈

 # 写在前面
- 本项目是在 [@gosip](https://github.com/ghettovoice/gosip) 的 项目基础上修改，根据自己的使用习惯修改添加，感谢！

# 使用

> go get -u github.com/zenghr0820/gsip

```go
package sip
import (
	"fmt"

	"github.com/zenghr0820/gsip"
	"github.com/zenghr0820/gsip/sip"
)

// 处理sip请求
var RequestHandleMap map[sip.RequestMethod]sip.RequestHandler
// 处理 SIP 响应
var ResponseHandle sip.ResponseHandler

// 初始化
func init() {
	// 请求
	RequestHandleMap = map[sip.RequestMethod]sip.RequestHandler{
		sip.REGISTER: func (request sip.Request, tx sip.ServerTransaction) {
			// ...
		},
		sip.MESSAGE:  func (request sip.Request, tx sip.ServerTransaction) {
			// ...
		},
		// ...
	}
	
	// 响应
	ResponseHandle = func (response sip.Response, tx sip.ClientTransaction) {
		// ...
	}
}

func InitSipServlet() {

	// 创建 SIP 服务
	service := gsip.NewService(
		gsip.Transport("192.168.0.102"),
		// 添加 sip 处理函数
		gsip.RequestCallback(RequestHandleMap),
		gsip.ResponseCallback(ResponseHandle),
		// 开启 sip 日志
		gsip.LoggerConfig(
			gsip.LoggerEnv("dev"),
			gsip.LoggerLevel("info"),
		),
	)

	// 监听 sip 端口
	if err := service.Listen("udp", fmt.Sprintf(
		"%s:%v",
		"0.0.0.0",
		5060)); err !=nil {
		fmt.Println(err)
	}

	if err := service.Listen("tcp", fmt.Sprintf(
		"%s:%v",
		"0.0.0.0",
		5060)); err !=nil {
		fmt.Println(err)
	}

	// 阻塞运行
	if err := service.Run(); err != nil {
		fmt.Println(err)
	}

}
```



 # 参考资料
 - [gosip](https://github.com/ghettovoice/gosip)

