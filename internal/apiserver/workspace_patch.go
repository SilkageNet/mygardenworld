package apiserver

import (
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"google.golang.org/protobuf/proto"
)

func buildWorkspacePatch(previous, next *pb.WorkspaceState) *pb.WorkspacePatch {
	if next == nil {
		return nil
	}
	patch := &pb.WorkspacePatch{Revision: next.GetRevision(), AccountId: next.GetAccountId()}
	if previous == nil || previous.GetAccountId() != next.GetAccountId() {
		patch.AccountStatus = next.GetAccountStatus()
		patch.Policy = next.GetPolicy()
		patch.Basic = next.GetBasic()
		patch.Garden = next.GetGarden()
		patch.Orders = next.GetOrders()
		patch.Union = next.GetUnion()
		patch.Activities = next.GetActivities()
		patch.Warehouse = next.GetWarehouse()
		patch.Statistics = next.GetStatistics()
		return patch
	}

	changed := previous.GetRevision() != next.GetRevision()
	if !proto.Equal(previous.GetAccountStatus(), next.GetAccountStatus()) {
		patch.AccountStatus = next.GetAccountStatus()
		changed = true
	}
	if !proto.Equal(previous.GetPolicy(), next.GetPolicy()) {
		patch.Policy = next.GetPolicy()
		changed = true
	}
	if !basicViewEqual(previous.GetBasic(), next.GetBasic()) {
		patch.Basic = next.GetBasic()
		changed = true
		appendClearedDomain(patch, pb.WorkspaceDomain_WORKSPACE_DOMAIN_BASIC, previous.GetBasic() != nil, next.GetBasic() != nil)
	}
	if !gardenViewEqual(previous.GetGarden(), next.GetGarden()) {
		patch.Garden = next.GetGarden()
		changed = true
		appendClearedDomain(patch, pb.WorkspaceDomain_WORKSPACE_DOMAIN_GARDEN, previous.GetGarden() != nil, next.GetGarden() != nil)
	}
	if !ordersViewEqual(previous.GetOrders(), next.GetOrders()) {
		patch.Orders = next.GetOrders()
		changed = true
		appendClearedDomain(patch, pb.WorkspaceDomain_WORKSPACE_DOMAIN_ORDERS, previous.GetOrders() != nil, next.GetOrders() != nil)
	}
	if !unionViewEqual(previous.GetUnion(), next.GetUnion()) {
		patch.Union = next.GetUnion()
		changed = true
		appendClearedDomain(patch, pb.WorkspaceDomain_WORKSPACE_DOMAIN_UNION, previous.GetUnion() != nil, next.GetUnion() != nil)
	}
	if !activitiesViewEqual(previous.GetActivities(), next.GetActivities()) {
		patch.Activities = next.GetActivities()
		changed = true
		appendClearedDomain(patch, pb.WorkspaceDomain_WORKSPACE_DOMAIN_ACTIVITIES, previous.GetActivities() != nil, next.GetActivities() != nil)
	}
	if !warehouseViewEqual(previous.GetWarehouse(), next.GetWarehouse()) {
		patch.Warehouse = next.GetWarehouse()
		changed = true
		appendClearedDomain(patch, pb.WorkspaceDomain_WORKSPACE_DOMAIN_WAREHOUSE, previous.GetWarehouse() != nil, next.GetWarehouse() != nil)
	}
	if !proto.Equal(previous.GetStatistics(), next.GetStatistics()) {
		patch.Statistics = next.GetStatistics()
		changed = true
		appendClearedDomain(patch, pb.WorkspaceDomain_WORKSPACE_DOMAIN_STATISTICS, previous.GetStatistics() != nil, next.GetStatistics() != nil)
	}
	if !changed {
		return nil
	}
	return patch
}

func appendClearedDomain(patch *pb.WorkspacePatch, domain pb.WorkspaceDomain, previousPresent, nextPresent bool) {
	if previousPresent && !nextPresent {
		patch.ClearedDomains = append(patch.ClearedDomains, domain)
	}
}

func basicViewEqual(left, right *pb.BasicView) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	a := proto.Clone(left).(*pb.BasicView)
	b := proto.Clone(right).(*pb.BasicView)
	a.CapturedAt = nil
	b.CapturedAt = nil
	return proto.Equal(a, b)
}

func gardenViewEqual(left, right *pb.GardenView) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	a := proto.Clone(left).(*pb.GardenView)
	b := proto.Clone(right).(*pb.GardenView)
	a.CapturedAt = nil
	b.CapturedAt = nil
	return proto.Equal(a, b)
}

func ordersViewEqual(left, right *pb.OrdersView) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	a := proto.Clone(left).(*pb.OrdersView)
	b := proto.Clone(right).(*pb.OrdersView)
	a.CapturedAt = nil
	b.CapturedAt = nil
	return proto.Equal(a, b)
}

func unionViewEqual(left, right *pb.UnionView) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	a := proto.Clone(left).(*pb.UnionView)
	b := proto.Clone(right).(*pb.UnionView)
	a.CapturedAt = nil
	b.CapturedAt = nil
	return proto.Equal(a, b)
}

func activitiesViewEqual(left, right *pb.ActivitiesView) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	a := proto.Clone(left).(*pb.ActivitiesView)
	b := proto.Clone(right).(*pb.ActivitiesView)
	a.CapturedAt = nil
	b.CapturedAt = nil
	return proto.Equal(a, b)
}

func warehouseViewEqual(left, right *pb.WarehouseView) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	a := proto.Clone(left).(*pb.WarehouseView)
	b := proto.Clone(right).(*pb.WarehouseView)
	a.CapturedAt = nil
	b.CapturedAt = nil
	return proto.Equal(a, b)
}
