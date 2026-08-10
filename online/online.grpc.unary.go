package main

import (
	"context"
	"strings"

	pb "server/proto/pb"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func (p *onlineGRPCServer) OnlineBindAccount(_ context.Context, req *pb.OnlineBindAccountReq) (*pb.OnlineBindAccountRes, error) {
	aid := req.GetAid()
	account := strings.TrimSpace(req.GetAccount())
	gatewayKey := strings.TrimSpace(req.GetGatewayKey())
	if aid == 0 || account == "" || gatewayKey == "" || req.GetAccountSession() == "" {
		return nil, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	accountRecord, err := unaryCacheGetAccountRecord(aid)
	if err != nil {
		if s, ok := grpcstatus.FromError(err); ok {
			return nil, grpcstatus.Error(s.Code(), s.Message())
		}
		return nil, grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if accountRecord == nil || accountRecord.GetAid() != aid || accountRecord.GetAccount() != account {
		return nil, grpcstatus.Error(grpccodes.Unauthenticated, "account record mismatch")
	}
	if err := validateAccountRecord(accountRecord); err != nil {
		return nil, grpcstatus.Errorf(grpccodes.Internal, "invalid account record: %v", err)
	}
	// protobuf 不保留空 map 的存在性, RPC 边界校验后统一恢复为可写空 map.
	if accountRecord.PetWarehouseRecordMap == nil {
		accountRecord.PetWarehouseRecordMap = make(map[uint64]*pb.PetRecord)
	}
	if accountRecord.ItemWarehouse == nil {
		accountRecord.ItemWarehouse = &pb.ItemContainerRecord{}
	}
	if accountRecord.ItemWarehouse.ItemCountMap == nil {
		accountRecord.ItemWarehouse.ItemCountMap = make(map[uint32]uint64)
	}
	if accountRecord.ItemWarehouse.EquipmentRecordMap == nil {
		accountRecord.ItemWarehouse.EquipmentRecordMap = make(map[uint64]*pb.EquipmentRecord)
	}
	for _, characterRecord := range accountRecord.GetCharacterRecordList() {
		if characterRecord == nil || characterRecord.GetBase().GetUuid() == 0 {
			continue
		}
		if characterRecord.ItemBag == nil {
			characterRecord.ItemBag = &pb.ItemContainerRecord{}
		}
		if characterRecord.ItemBag.ItemCountMap == nil {
			characterRecord.ItemBag.ItemCountMap = make(map[uint32]uint64)
		}
		if characterRecord.ItemBag.EquipmentRecordMap == nil {
			characterRecord.ItemBag.EquipmentRecordMap = make(map[uint64]*pb.EquipmentRecord)
		}
		if characterRecord.AssetCountMap == nil {
			characterRecord.AssetCountMap = make(map[uint32]uint64)
		} else {
			for assetID, count := range characterRecord.AssetCountMap {
				if count == 0 {
					delete(characterRecord.AssetCountMap, assetID)
				}
			}
		}
	}
	req.Account = account
	req.GatewayKey = gatewayKey
	res, err := GAccountMgr.Bind(aid, req, accountRecord)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, grpcstatus.Error(grpccodes.Internal, "online bind response is empty")
	}
	return res, nil
}

func (p *onlineGRPCServer) OnlineUnbindAccount(_ context.Context, req *pb.OnlineUnbindAccountReq) (*pb.OnlineUnbindAccountRes, error) {
	if req.GetAid() == 0 || req.GetGatewayKey() == "" || req.GetAccountSession() == "" {
		return &pb.OnlineUnbindAccountRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	account, ok := GAccountMgr.accounts.Find(req.GetAid())
	if !ok {
		return &pb.OnlineUnbindAccountRes{}, nil
	}
	account.PostUnbind(req.GetGatewayKey(), req.GetAccountSession())
	return &pb.OnlineUnbindAccountRes{}, nil
}
