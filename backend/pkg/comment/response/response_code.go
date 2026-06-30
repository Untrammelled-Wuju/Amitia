// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package response

const (
	OK   = 200
	FAIL = 400

	InvalidParams   = 400
	Unauthorized    = 401
	Forbidden       = 403
	NotFound        = 404
	TooManyRequests = 429

	InternalError = 500

	BusinessError   = 600
	DataNotFound    = 601
	OperationFailed = 602

	InvalidToken = 700
	TokenExpired = 701
	AccessDenied = 702
)

func GetMessage(code int) string {
	switch code {
	case OK:
		return "操作成功"
	case InvalidParams:
		return "参数错误"
	case Unauthorized:
		return "未授权，请先登录"
	case Forbidden:
		return "无权限访问"
	case NotFound:
		return "资源不存在"
	case TooManyRequests:
		return "请求过于频繁"
	case InternalError:
		return "服务器内部错误"
	case BusinessError:
		return "业务处理失败"
	case DataNotFound:
		return "数据不存在"
	case OperationFailed:
		return "操作失败"
	case InvalidToken:
		return "无效的令牌"
	case TokenExpired:
		return "令牌已过期"
	case AccessDenied:
		return "访问被拒绝"
	default:
		return "未知错误"
	}
}
