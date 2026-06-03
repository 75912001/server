package main

import (
	"fmt"
	servercommon "server/common"

	xerror "github.com/75912001/xlib/error"
	xpacket "github.com/75912001/xlib/packet"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var apiYamlPath string
var resultErrorMap = buildResultErrorMap()

func marshalJSON(msg proto.Message) string {
	data, err := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: true,
	}.Marshal(msg)
	if err != nil {
		return fmt.Sprintf(`{"error":"%v"}`, err)
	}
	return string(data)
}

func marshalHeaderMap(header *xpacket.Header) map[string]any {
	return map[string]any{
		"MessageID": fmt.Sprintf("0x%x", header.MessageID),
		"Length":    header.Length,
		"SessionID": header.SessionID,
		"ResultID":  header.ResultID,
		"Key":       header.Key,
	}
}

func buildResultErrorMap() map[uint32]*xerror.Error {
	out := make(map[uint32]*xerror.Error)
	registerResultErrors(out,
		xerror.Success,
		xerror.Fail,
		xerror.NotSupport,
		xerror.Cancelled,
		xerror.InvalidArgument,
		xerror.DeadlineExceeded,
		xerror.NotFound,
		xerror.AlreadyExists,
		xerror.PermissionDenied,
		xerror.ResourceExhausted,
		xerror.FailedPrecondition,
		xerror.Aborted,
		xerror.OutOfRange,
		xerror.Unimplemented,
		xerror.Internal,
		xerror.Unavailable,
		xerror.DataLoss,
		xerror.Unauthenticated,
		xerror.Available,
		xerror.Valid,
		xerror.Invalid,
		xerror.Legal,
		xerror.Illegal,
		xerror.Permitted,
		xerror.Prohibited,
		xerror.Expect,
		xerror.Unexpected,
		xerror.Enable,
		xerror.Disable,
		xerror.Normal,
		xerror.Abnormal,
		xerror.NotTimeout,
		xerror.Timeout,
		xerror.NotOutOfRange,
		xerror.NotConflict,
		xerror.Conflict,
		xerror.Matched,
		xerror.Mismatch,
		xerror.Implemented,
		xerror.Registered,
		xerror.Unregistered,
		xerror.Marshal,
		xerror.Unmarshal,
		xerror.Level,
		xerror.LevelNotEnough,
		xerror.NotDuplicate,
		xerror.Duplicate,
		xerror.Idle,
		xerror.Busy,
		xerror.AdequateResources,
		xerror.OutOfResources,
		xerror.ValidOperation,
		xerror.InvalidOperation,
		xerror.AdequateCondition,
		xerror.IllConditioned,
		xerror.PermissionGranted,
		xerror.NotFrozen,
		xerror.Frozen,
		xerror.Hit,
		xerror.Miss,
		xerror.Length,
		xerror.LengthNotEnough,
		xerror.Quantity,
		xerror.Retry,
		xerror.Link,
		xerror.System,
		xerror.Param,
		xerror.ParamNotSupport,
		xerror.ParamCountNotMatch,
		xerror.Packet,
		xerror.Config,
		xerror.Overload,
		xerror.Nil,
		xerror.Format,
		xerror.InterfaceNotMatch,
		xerror.NoBehavior,
		xerror.Disconnect,
		xerror.GoroutinePanic,
		xerror.GoroutineDone,
		xerror.FunctionPanic,
		xerror.FunctionDone,
		xerror.ChannelFull,
		xerror.ChannelEmpty,
		xerror.ChannelNotClosed,
		xerror.ChannelClosed,
		xerror.ChannelNil,
		xerror.GRPCNotFoundShardKey,
		xerror.GRPCInvalidMethod,
		xerror.GRPCNotSupportShardKeyType,
		xerror.GRPCServiceNotFound,
		xerror.Insert,
		xerror.Find,
		xerror.Update,
		xerror.Delete,
		xerror.Send,
		xerror.Receive,
		xerror.Configure,
		xerror.Unknown,
		servercommon.ECGatewayOnlineNotFound,
		servercommon.ECCacheInvalidArgument,
		servercommon.ECCacheKeyNotFound,
		servercommon.ECCacheRedisError,
	)
	return out
}

func registerResultErrors(dst map[uint32]*xerror.Error, errs ...*xerror.Error) {
	for _, errCode := range errs {
		dst[errCode.Code()] = errCode
	}
}

func formatResultError(code uint32) string {
	errCode, ok := resultErrorMap[code]
	if !ok {
		if _, registered := xerror.ErrMap.Find(code); registered {
			return fmt.Sprintf("code=%d 0x%X name=<registered> desc=<registered without detail>", code, code)
		}
		return fmt.Sprintf("code=%d 0x%X name=<unknown> desc=<unknown>", code, code)
	}
	return fmt.Sprintf("code=%d 0x%X name=%s desc=%s", code, code, errCode.Name(), errCode.Desc())
}
