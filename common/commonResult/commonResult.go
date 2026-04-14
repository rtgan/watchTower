package commonResult

import "watchTower/common/enum"

type BizCode = int

const SuccessCode BizCode = 200 //根据前端需要配置即可

var SuccessMap = enum.Enum{
	SuccessCode: "success",
}

// # 服务错误码与常见原因
const (
	ParamErrorCode       BizCode = 12001 // 调用接口参数错误（如:类型不匹配）
	DBUpdateErrorCode    BizCode = 12002 // 数据库更新失败（数据库的问题，建议排查日志）
	DownstreamErrorCode  BizCode = 12003 // 下游接口错误（下游接口报错）
	InstanceNotExistCode BizCode = 12004 // 实例不存在（上游传了一个错误的实例id）
)

var ErrorMap = enum.Enum{
	ParamErrorCode:       "param_error",
	DBUpdateErrorCode:    "db_update_error",
	DownstreamErrorCode:  "downstream_error",
	InstanceNotExistCode: "instance_not_exist",
}

type CommonResult struct {
	Code    BizCode `json:"code"`
	Message string  `json:"message"`
	Data    any     `json:"data,omitempty"`
}

func New() *CommonResult {
	return &CommonResult{}
}

func (r *CommonResult) Deal(data any, err error) *CommonResult {
	if err != nil {
		code := ErrorMap.Code(err.Error())
		if code != -1 {
			r.Fail(code, err)
			return r
		}
		r.Fail(500, err)
	}
	r.Success(data)

	return r
}

func (r *CommonResult) Success(data any) {
	r.Code = SuccessCode
	r.Message = SuccessMap.Value(SuccessCode)
	r.Data = data
}

func (r *CommonResult) Fail(code BizCode, err error) {
	r.Code = code
	r.Message = err.Error()
	r.Data = nil
}
