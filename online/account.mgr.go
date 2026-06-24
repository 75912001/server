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

func (p *AccountMgr) GetByUID(uid uint64) *Account {
	account, ok := p.accounts.Find(uid)
	if !ok {
		return nil
	}
	return account
}

func (p *AccountMgr) Bind(uid uint64, req *pb.OnlineBindUserReq, accountRecord *pb.AccountRecord) (*pb.OnlineBindUserRes, error) {
	account, existed := p.accounts.Find(uid)
	if !existed {
		account = newAccount(uid)
		p.accounts.Add(uid, account)
	}
	res, err := account.PostBind(req, accountRecord)
	if err != nil {
		current, ok := p.accounts.Find(uid)
		if !existed {
			if !ok || current == account {
				p.accounts.Del(uid)
			}
			account.Stop()
		} else if !ok {
			account.Stop()
		}
		return nil, err
	}
	return res, nil
}
