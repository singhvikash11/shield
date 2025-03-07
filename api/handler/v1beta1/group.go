package v1beta1

import (
	"context"
	"errors"
	"strings"

	grpczap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"

	"github.com/odpf/shield/internal/group"
	"github.com/odpf/shield/model"

	shieldv1beta1 "github.com/odpf/shield/proto/v1beta1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GroupService interface {
	CreateGroup(ctx context.Context, grp model.Group) (model.Group, error)
	GetGroup(ctx context.Context, id string) (model.Group, error)
	ListGroups(ctx context.Context, org model.Organization) ([]model.Group, error)
	UpdateGroup(ctx context.Context, grp model.Group) (model.Group, error)
	AddUsersToGroup(ctx context.Context, groupId string, userIds []string) ([]model.User, error)
	ListGroupUsers(ctx context.Context, groupId string) ([]model.User, error)
	ListGroupAdmins(ctx context.Context, groupId string) ([]model.User, error)
	RemoveUserFromGroup(ctx context.Context, groupId string, userId string) ([]model.User, error)
}

var (
	grpcGroupNotFoundErr = status.Errorf(codes.NotFound, "group doesn't exist")
)

func (v Dep) ListGroups(ctx context.Context, request *shieldv1beta1.ListGroupsRequest) (*shieldv1beta1.ListGroupsResponse, error) {
	logger := grpczap.Extract(ctx)

	var groups []*shieldv1beta1.Group

	groupList, err := v.GroupService.ListGroups(ctx, model.Organization{Id: request.OrgId})
	if errors.Is(err, group.GroupDoesntExist) {
		return nil, nil
	} else if err != nil {
		logger.Error(err.Error())
		return nil, grpcInternalServerError
	}

	for _, v := range groupList {
		groupPB, err := transformGroupToPB(v)
		if err != nil {
			logger.Error(err.Error())
			return nil, grpcInternalServerError
		}

		groups = append(groups, &groupPB)
	}

	return &shieldv1beta1.ListGroupsResponse{Groups: groups}, nil
}

func (v Dep) CreateGroup(ctx context.Context, request *shieldv1beta1.CreateGroupRequest) (*shieldv1beta1.CreateGroupResponse, error) {
	logger := grpczap.Extract(ctx)

	metaDataMap, err := mapOfStringValues(request.GetBody().Metadata.AsMap())
	if err != nil {
		logger.Error(err.Error())
		return nil, grpcBadBodyError
	}

	slug := request.GetBody().Slug
	if strings.TrimSpace(slug) == "" {
		slug = generateSlug(request.GetBody().Name)
	}

	newGroup, err := v.GroupService.CreateGroup(ctx, model.Group{
		Name:           request.Body.Name,
		Slug:           slug,
		OrganizationId: request.Body.OrgId,
		Metadata:       metaDataMap,
	})

	if err != nil {
		logger.Error(err.Error())
		return nil, grpcInternalServerError
	}

	metaData, err := structpb.NewStruct(mapOfInterfaceValues(newGroup.Metadata))
	if err != nil {
		logger.Error(err.Error())
		return nil, grpcInternalServerError
	}

	return &shieldv1beta1.CreateGroupResponse{Group: &shieldv1beta1.Group{
		Id:        newGroup.Id,
		Name:      newGroup.Name,
		Slug:      newGroup.Slug,
		OrgId:     newGroup.Organization.Id,
		Metadata:  metaData,
		CreatedAt: timestamppb.New(newGroup.CreatedAt),
		UpdatedAt: timestamppb.New(newGroup.UpdatedAt),
	}}, nil
}

func (v Dep) GetGroup(ctx context.Context, request *shieldv1beta1.GetGroupRequest) (*shieldv1beta1.GetGroupResponse, error) {
	logger := grpczap.Extract(ctx)

	fetchedGroup, err := v.GroupService.GetGroup(ctx, request.GetId())
	if err != nil {
		logger.Error(err.Error())
		switch {
		case errors.Is(err, group.GroupDoesntExist):
			return nil, grpcGroupNotFoundErr
		case errors.Is(err, group.InvalidUUID):
			return nil, grpcBadBodyError
		default:
			return nil, grpcInternalServerError
		}
	}

	groupPB, err := transformGroupToPB(fetchedGroup)
	if err != nil {
		logger.Error(err.Error())
		return nil, grpcInternalServerError
	}

	return &shieldv1beta1.GetGroupResponse{Group: &groupPB}, nil
}

func (v Dep) ListGroupUsers(ctx context.Context, request *shieldv1beta1.ListGroupUsersRequest) (*shieldv1beta1.ListGroupUsersResponse, error) {
	logger := grpczap.Extract(ctx)

	usersList, err := v.GroupService.ListGroupUsers(ctx, request.GetId())

	if err != nil {
		logger.Error(err.Error())
		return nil, grpcInternalServerError
	}

	var users []*shieldv1beta1.User

	for _, u := range usersList {
		userPB, err := transformUserToPB(u)
		if err != nil {
			logger.Error(err.Error())
			return nil, grpcInternalServerError
		}
		users = append(users, &userPB)
	}

	return &shieldv1beta1.ListGroupUsersResponse{
		Users: users,
	}, nil
}

func (v Dep) AddGroupUser(ctx context.Context, request *shieldv1beta1.AddGroupUserRequest) (*shieldv1beta1.AddGroupUserResponse, error) {
	logger := grpczap.Extract(ctx)
	if request.Body == nil {
		return nil, grpcBadBodyError
	}
	updatedUsers, err := v.GroupService.AddUsersToGroup(ctx, request.GetId(), request.GetBody().UserIds)

	if err != nil {
		logger.Error(err.Error())
		switch {
		case errors.Is(err, group.GroupDoesntExist):
			return nil, status.Errorf(codes.NotFound, "group to be updated not found")
		default:
			return nil, grpcInternalServerError
		}
	}

	var users []*shieldv1beta1.User

	for _, u := range updatedUsers {
		userPB, err := transformUserToPB(u)
		if err != nil {
			logger.Error(err.Error())
			return nil, grpcInternalServerError
		}
		users = append(users, &userPB)
	}

	return &shieldv1beta1.AddGroupUserResponse{
		Users: users,
	}, nil
}

func (v Dep) RemoveGroupUser(ctx context.Context, request *shieldv1beta1.RemoveGroupUserRequest) (*shieldv1beta1.RemoveGroupUserResponse, error) {
	logger := grpczap.Extract(ctx)
	_, err := v.GroupService.RemoveUserFromGroup(ctx, request.GetId(), request.GetUserId())

	if err != nil {
		logger.Error(err.Error())
		switch {
		case errors.Is(err, group.GroupDoesntExist):
			return nil, status.Errorf(codes.NotFound, "group to be updated not found")
		default:
			return nil, grpcInternalServerError
		}
	}

	return &shieldv1beta1.RemoveGroupUserResponse{
		Message: "Removed User from group",
	}, nil
}

func (v Dep) UpdateGroup(ctx context.Context, request *shieldv1beta1.UpdateGroupRequest) (*shieldv1beta1.UpdateGroupResponse, error) {
	logger := grpczap.Extract(ctx)

	if request.Body == nil {
		return nil, grpcBadBodyError
	}

	metaDataMap, err := mapOfStringValues(request.GetBody().Metadata.AsMap())
	if err != nil {
		return nil, grpcBadBodyError
	}

	updatedGroup, err := v.GroupService.UpdateGroup(ctx, model.Group{
		Id:           request.GetId(),
		Name:         request.GetBody().GetName(),
		Slug:         request.GetBody().GetSlug(),
		Organization: model.Organization{Id: request.GetBody().OrgId},
		Metadata:     metaDataMap,
	})
	if err != nil {
		logger.Error(err.Error())
		switch {
		case errors.Is(err, group.GroupDoesntExist):
			return nil, status.Errorf(codes.NotFound, "group to be updated not found")
		default:
			return nil, grpcInternalServerError
		}
	}

	groupPB, err := transformGroupToPB(updatedGroup)
	if err != nil {
		return nil, grpcInternalServerError
	}

	return &shieldv1beta1.UpdateGroupResponse{Group: &groupPB}, nil
}

func (v Dep) ListGroupAdmins(ctx context.Context, request *shieldv1beta1.ListGroupAdminsRequest) (*shieldv1beta1.ListGroupAdminsResponse, error) {
	logger := grpczap.Extract(ctx)
	usersList, err := v.GroupService.ListGroupAdmins(ctx, request.GetId())

	if err != nil {
		logger.Error(err.Error())
		return nil, grpcInternalServerError
	}

	var users []*shieldv1beta1.User

	for _, u := range usersList {
		userPB, err := transformUserToPB(u)
		if err != nil {
			logger.Error(err.Error())
			return nil, grpcInternalServerError
		}
		users = append(users, &userPB)
	}

	return &shieldv1beta1.ListGroupAdminsResponse{
		Users: users,
	}, nil
}

func transformGroupToPB(grp model.Group) (shieldv1beta1.Group, error) {
	metaData, err := structpb.NewStruct(mapOfInterfaceValues(grp.Metadata))
	if err != nil {
		return shieldv1beta1.Group{}, err
	}

	return shieldv1beta1.Group{
		Id:        grp.Id,
		Name:      grp.Name,
		Slug:      grp.Slug,
		OrgId:     grp.Organization.Id,
		Metadata:  metaData,
		CreatedAt: timestamppb.New(grp.CreatedAt),
		UpdatedAt: timestamppb.New(grp.UpdatedAt),
	}, nil
}
