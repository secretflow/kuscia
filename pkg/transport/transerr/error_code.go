// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transerr

type TransError struct {
	code ErrorCode
}

func (err *TransError) Error() string {
	return string(err.code)
}

func (err *TransError) ErrorInfo() string {
	return errInfoMap[err.code]
}

func NewTransError(c ErrorCode) *TransError {
	return &TransError{
		code: c,
	}
}

func GetErrorInfo(c ErrorCode) string {
	return errInfoMap[c]
}

type ErrorCode string

const (
	Success                    ErrorCode = "E0000000000"
	InvalidRequest             ErrorCode = "E0000000400"
	UnAuthorizedResource       ErrorCode = "E0000000403"
	NotFound                   ErrorCode = "E0000000404"
	BodyTooLarge               ErrorCode = "E0000000413"
	ServerError                ErrorCode = "E0000000500"
	ServerUnavailable          ErrorCode = "E0000000503"
	UnknownError               ErrorCode = "E0000000520"
	SystemIncompatible         ErrorCode = "E0000000600"
	RequestTimeout             ErrorCode = "E0000000601"
	NoServerInstance           ErrorCode = "E0000000602"
	InvalidCertificate         ErrorCode = "E0000000603"
	DomainAuthorizationTimeout ErrorCode = "E0000000604"
	DomainNetworkTimeout       ErrorCode = "E0000000605"
	PeerDisableNetwork         ErrorCode = "E0000000606"
	NetworkUnreachable         ErrorCode = "E0000000607"
	SessionReleased            ErrorCode = "E0000000608"
	UnAuthorizedInterface      ErrorCode = "E0000000614"
	InvalidSignature           ErrorCode = "E0000000615"
	CodecError                 ErrorCode = "E0000000616"
	DownStreamUnExist          ErrorCode = "E0000000617"
	DomainNotInNetwork         ErrorCode = "E0000000618"
	AddressUnreachable         ErrorCode = "E0000000619"
	BufferOverflow             ErrorCode = "E0000000700"
)

var errInfoMap = map[ErrorCode]string{
	Success:                    "请求成功",
	InvalidRequest:             "请求非法",
	UnAuthorizedResource:       "请求资源未被授权",
	NotFound:                   "请求资源不存在",
	BodyTooLarge:               "请求报文超长",
	ServerError:                "系统异常",
	ServerUnavailable:          "循环请求服务不可达",
	UnknownError:               "未知异常",
	SystemIncompatible:         "系统不兼容",
	RequestTimeout:             "请求超时",
	NoServerInstance:           "无服务实例",
	InvalidCertificate:         "数字证书校验异常",
	DomainAuthorizationTimeout: "节点授权码已过期",
	DomainNetworkTimeout:       "节点组网时间已过期",
	PeerDisableNetwork:         "对方节点已禁用网络",
	NetworkUnreachable:         "网络不通",
	SessionReleased:            "会话已释放",
	UnAuthorizedInterface:      "接口未被许可调用",
	InvalidSignature:           "证书签名非法",
	CodecError:                 "报文编解码异常",
	DownStreamUnExist:          "下游版本不匹配服务不存在",
	DomainNotInNetwork:         "节点或机构未组网",
	AddressUnreachable:         "地址非法或无法访问",
	BufferOverflow:             "缓冲区已满",
}
