package main

import (
	"errors"
	"fmt"
	"math"

	"server/common/gameconfig"
	pb "server/proto/pb"
)

var (
	errStoreCostInvalid      = errors.New("store cost is invalid")
	errStoreCostInsufficient = errors.New("store cost is insufficient")
)

type storeCostAmount struct {
	itemID uint32
	count  uint64
}

// prepareStoreCostAmounts 在修改候选档案前计算并校验全部消耗, 避免部分扣除后才发现余额不足.
func prepareStoreCostAmounts(entry *gameconfig.StoreEntry, quantity uint32, characterRecord *pb.CharacterRecord) ([]storeCostAmount, error) {
	if entry == nil || entry.Costs == nil || quantity == 0 || characterRecord == nil {
		return nil, errStoreCostInvalid
	}
	amounts := make([]storeCostAmount, 0, len(entry.GetCosts()))
	seenItemIDs := make(map[uint32]struct{}, len(entry.GetCosts()))
	itemManager := newCharacterItemManager(characterRecord)
	for index, cost := range entry.GetCosts() {
		if cost == nil || cost.ItemID == nil || *cost.ItemID == 0 || cost.Count == nil || *cost.Count == 0 {
			return nil, fmt.Errorf("%w: index %d", errStoreCostInvalid, index)
		}
		if *cost.Count > math.MaxUint64/uint64(quantity) {
			return nil, fmt.Errorf("%w: item %d count overflows", errStoreCostInvalid, *cost.ItemID)
		}
		if _, exists := seenItemIDs[*cost.ItemID]; exists {
			return nil, fmt.Errorf("%w: duplicate item %d", errStoreCostInvalid, *cost.ItemID)
		}
		seenItemIDs[*cost.ItemID] = struct{}{}
		amount := *cost.Count * uint64(quantity)
		if itemManager.Count(*cost.ItemID) < amount {
			return nil, fmt.Errorf("%w: item %d needs %d", errStoreCostInsufficient, *cost.ItemID, amount)
		}
		amounts = append(amounts, storeCostAmount{itemID: *cost.ItemID, count: amount})
	}
	return amounts, nil
}

// consumeStoreCostAmounts 在候选角色档案中扣除全部资源并返回每项最终权威余额.
func consumeStoreCostAmounts(characterRecord *pb.CharacterRecord, amounts []storeCostAmount) ([]*pb.ItemCostResult, error) {
	if characterRecord == nil {
		return nil, errStoreCostInvalid
	}
	itemManager := newCharacterItemManager(characterRecord)
	results := make([]*pb.ItemCostResult, 0, len(amounts))
	for _, amount := range amounts {
		if amount.itemID == 0 || amount.count == 0 {
			return nil, errStoreCostInvalid
		}
		if err := itemManager.Consume(amount.itemID, amount.count); err != nil {
			return nil, fmt.Errorf("%w: consume item %d: %v", errStoreCostInvalid, amount.itemID, err)
		}
		results = append(results, &pb.ItemCostResult{
			ItemId:         amount.itemID,
			ConsumedCount:  amount.count,
			RemainingCount: itemManager.Count(amount.itemID),
		})
	}
	return results, nil
}

func cloneItemCostResults(results []*pb.ItemCostResult) []*pb.ItemCostResult {
	clones := make([]*pb.ItemCostResult, 0, len(results))
	for _, result := range results {
		if result != nil {
			clones = append(clones, &pb.ItemCostResult{
				ItemId:         result.GetItemId(),
				ConsumedCount:  result.GetConsumedCount(),
				RemainingCount: result.GetRemainingCount(),
			})
		}
	}
	return clones
}
