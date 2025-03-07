package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/odpf/shield/internal/resource"
	"github.com/odpf/shield/model"
)

type Resource struct {
	Id             string         `db:"id"`
	Name           string         `db:"name"`
	ProjectId      string         `db:"project_id"`
	Project        Project        `db:"project"`
	GroupId        sql.NullString `db:"group_id"`
	Group          Group          `db:"group"`
	OrganizationId string         `db:"org_id"`
	Organization   Organization   `db:"organization"`
	NamespaceId    string         `db:"namespace_id"`
	Namespace      Namespace      `db:"namespace"`
	User           User           `db:"user"`
	UserId         sql.NullString `db:"user_id"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
}

const (
	createResourceQuery = `
		INSERT INTO resources (
			id,
		    name,
			project_id,
			group_id,
			org_id,
			namespace_id,
		    user_id
		) VALUES (
			$1,
			$2,
			$3,
			$4,
			$5,
		    $6,
		    $7
		)
		RETURNING id, name, project_id, group_id, org_id, namespace_id, user_id, created_at, updated_at`
	listResourcesQuery = `
		SELECT
			id,
		    name,
			project_id,
			group_id,
			org_id,
			namespace_id,
		    user_id,
			created_at,
			updated_at
		FROM resources`
	getResourcesQuery = `
		SELECT
			id,
		    name,
			project_id,
			group_id,
			org_id,
			namespace_id,
		    user_id,
			created_at,
			updated_at
		FROM resources
		WHERE id = $1`
	updateResourceQuery = `
		UPDATE resources SET
		    name = $2,
			project_id = $3,
			group_id = $4,
			org_id = $5,
			namespace_id = $6,
		    user_id = $7
		WHERE id = $1
		`
)

func (s Store) CreateResource(ctx context.Context, resourceToCreate model.Resource) (model.Resource, error) {
	var newResource Resource

	userId := sql.NullString{String: resourceToCreate.UserId, Valid: resourceToCreate.UserId != ""}
	groupId := sql.NullString{String: resourceToCreate.GroupId, Valid: resourceToCreate.GroupId != ""}

	err := s.DB.WithTimeout(ctx, func(ctx context.Context) error {
		return s.DB.GetContext(ctx, &newResource, createResourceQuery, resourceToCreate.Id, resourceToCreate.Name, resourceToCreate.ProjectId, groupId, resourceToCreate.OrganizationId, resourceToCreate.NamespaceId, userId)
	})

	if err != nil {
		return model.Resource{}, err
	}

	transformedResource, err := transformToResource(newResource)

	if err != nil {
		return model.Resource{}, err
	}

	return transformedResource, nil
}

func (s Store) ListResources(ctx context.Context) ([]model.Resource, error) {
	var fetchedResources []Resource
	err := s.DB.WithTimeout(ctx, func(ctx context.Context) error {
		return s.DB.SelectContext(ctx, &fetchedResources, listResourcesQuery)
	})

	if errors.Is(err, sql.ErrNoRows) {
		return []model.Resource{}, resource.ResourceDoesntExist
	}

	if err != nil {
		return []model.Resource{}, fmt.Errorf("%w: %s", dbErr, err)
	}

	var transformedResources []model.Resource

	for _, r := range fetchedResources {
		transformedResource, err := transformToResource(r)
		if err != nil {
			return []model.Resource{}, fmt.Errorf("%w: %s", parseErr, err)
		}

		transformedResources = append(transformedResources, transformedResource)
	}

	return transformedResources, nil
}

func (s Store) GetResource(ctx context.Context, id string) (model.Resource, error) {
	var fetchedResource Resource
	err := s.DB.WithTimeout(ctx, func(ctx context.Context) error {
		return s.DB.GetContext(ctx, &fetchedResource, getResourcesQuery, id)
	})

	if errors.Is(err, sql.ErrNoRows) {
		return model.Resource{}, resource.ResourceDoesntExist
	} else if err != nil && fmt.Sprintf("%s", err.Error()[0:38]) == "pq: invalid input syntax for type uuid" {
		// TODO: this uuid syntax is a error defined in db, not in library
		// need to look into better ways to implement this
		return model.Resource{}, resource.InvalidUUID
	} else if err != nil {
		return model.Resource{}, fmt.Errorf("%w: %s", dbErr, err)
	}

	if err != nil {
		return model.Resource{}, err
	}

	transformedResource, err := transformToResource(fetchedResource)
	if err != nil {
		return model.Resource{}, err
	}

	return transformedResource, nil
}

func (s Store) UpdateResource(ctx context.Context, id string, toUpdate model.Resource) (model.Resource, error) {
	var updatedResource Resource

	userId := sql.NullString{String: toUpdate.UserId, Valid: toUpdate.UserId != ""}
	groupId := sql.NullString{String: toUpdate.GroupId, Valid: toUpdate.GroupId != ""}

	err := s.DB.WithTimeout(ctx, func(ctx context.Context) error {
		return s.DB.GetContext(ctx, &updatedResource, updateResourceQuery, id, toUpdate.Name, toUpdate.ProjectId, groupId, toUpdate.OrganizationId, toUpdate.NamespaceId, userId)
	})

	if errors.Is(err, sql.ErrNoRows) {
		return model.Resource{}, resource.ResourceDoesntExist
	} else if err != nil && fmt.Sprintf("%s", err.Error()[0:38]) == "pq: invalid input syntax for type uuid" {
		// TODO: this uuid syntax is a error defined in db, not in library
		// need to look into better ways to implement this
		return model.Resource{}, fmt.Errorf("%w: %s", resource.InvalidUUID, err)
	} else if err != nil {
		return model.Resource{}, fmt.Errorf("%w: %s", dbErr, err)
	}

	toUpdate, err = transformToResource(updatedResource)
	if err != nil {
		return model.Resource{}, fmt.Errorf("%s: %w", parseErr, err)
	}

	return toUpdate, nil
}

func transformToResource(from Resource) (model.Resource, error) {
	// TODO: remove *Id
	return model.Resource{
		Id:             from.Id,
		Name:           from.Name,
		Project:        model.Project{Id: from.ProjectId},
		ProjectId:      from.ProjectId,
		Namespace:      model.Namespace{Id: from.NamespaceId},
		NamespaceId:    from.NamespaceId,
		Organization:   model.Organization{Id: from.OrganizationId},
		OrganizationId: from.OrganizationId,
		Group:          model.Group{Id: from.GroupId.String},
		User:           model.User{Id: from.UserId.String},
		CreatedAt:      from.CreatedAt,
		UpdatedAt:      from.UpdatedAt,
	}, nil
}
