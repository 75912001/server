package main

import (
	pb "server/proto/pb"

	xmap "github.com/75912001/xlib/map"
)

var GAccountMgr = &AccountMgr{
	accounts: xmap.NewMapMutexMgr[uint64, *Account](),
}

type AccountMgr struct {
	accounts *xmap.MapMutexMgr[uint64, *Account]
}

func (p *AccountMgr) GetByAID(aid uint64) *Account {
	account, ok := p.accounts.Find(aid)
	if !ok {
		return nil
	}
	return account
}

func (p *AccountMgr) Bind(aid uint64, req *pb.OnlineBindAccountReq, accountRecord *pb.AccountRecord) (*pb.OnlineBindAccountRes, error) {
	account, existed := p.accounts.Find(aid)
	if !existed {
		account = newAccount(aid)
		p.accounts.Add(aid, account)
	}
	res, err := account.PostBind(req, accountRecord)
	if err != nil {
		current, ok := p.accounts.Find(aid)
		if !existed {
			if !ok || current == account {
				p.accounts.Del(aid)
			}
			account.Stop()
		} else if !ok {
			account.Stop()
		}
		return nil, err
	}
	return res, nil
}
