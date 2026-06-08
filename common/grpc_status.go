package common

import (
	"net/http"

	xerror "github.com/75912001/xlib/error"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func GRPCCodeToResultID(code codes.Code) uint32 {
	switch code {
	case codes.OK:
		return xerror.Success.Code()
	case codes.Canceled:
		return xerror.Cancelled.Code()
	case codes.Unknown:
		return xerror.Unknown.Code()
	case codes.InvalidArgument:
		return xerror.InvalidArgument.Code()
	case codes.DeadlineExceeded:
		return xerror.DeadlineExceeded.Code()
	case codes.NotFound:
		return xerror.NotFound.Code()
	case codes.AlreadyExists:
		return xerror.AlreadyExists.Code()
	case codes.PermissionDenied:
		return xerror.PermissionDenied.Code()
	case codes.ResourceExhausted:
		return xerror.ResourceExhausted.Code()
	case codes.FailedPrecondition:
		return xerror.FailedPrecondition.Code()
	case codes.Aborted:
		return xerror.Aborted.Code()
	case codes.OutOfRange:
		return xerror.OutOfRange.Code()
	case codes.Unimplemented:
		return xerror.Unimplemented.Code()
	case codes.Internal:
		return xerror.Internal.Code()
	case codes.Unavailable:
		return xerror.Unavailable.Code()
	case codes.DataLoss:
		return xerror.DataLoss.Code()
	case codes.Unauthenticated:
		return xerror.Unauthenticated.Code()
	default:
		return xerror.Fail.Code()
	}
}

func GRPCStatusToResultID(err error) uint32 {
	if err == nil {
		return xerror.Success.Code()
	}
	status, ok := grpcstatus.FromError(err)
	if !ok {
		return xerror.Fail.Code()
	}
	return GRPCCodeToResultID(status.Code())
}

func GRPCCodeToHTTPStatus(code codes.Code) int {
	switch code {
	case codes.InvalidArgument, codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusUnauthorized
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.FailedPrecondition, codes.Aborted:
		return http.StatusConflict
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.Internal, codes.DataLoss, codes.Unknown:
		return http.StatusBadGateway
	default:
		return http.StatusBadGateway
	}
}

func GRPCStatusToHTTP(err error, fallbackMessage string) (int, string) {
	if err == nil {
		return http.StatusOK, ""
	}
	status, ok := grpcstatus.FromError(err)
	if !ok {
		return http.StatusServiceUnavailable, fallbackMessage
	}
	return GRPCCodeToHTTPStatus(status.Code()), status.Message()
}
