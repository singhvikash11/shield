package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/odpf/shield/internal/project"
	"github.com/odpf/shield/model"
)

type Project struct {
	Id        string    `db:"id"`
	Name      string    `db:"name"`
	Slug      string    `db:"slug"`
	OrgId     string    `db:"org_id"`
	Metadata  []byte    `db:"metadata"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

const (
	getProjectsQuery   = `SELECT id, name, slug, org_id, metadata, created_at, updated_at from projects where id=$1;`
	createProjectQuery = `INSERT INTO projects(name, slug, org_id, metadata) values($1, $2, $3, $4) RETURNING id, name, slug, org_id, metadata, created_at, updated_at;`
	listProjectQuery   = `SELECT id, name, slug, org_id, metadata, created_at, updated_at from projects;`
	updateProjectQuery = `UPDATE projects set name = $2, slug = $3, org_id=$4, metadata = $5, updated_at = now() where id = $1 RETURNING id, name, slug, metadata, created_at, updated_at;`
)

func (s Store) GetProject(ctx context.Context, id string) (model.Project, error) {
	var fetchedProject Project
	err := s.DB.WithTimeout(ctx, func(ctx context.Context) error {
		return s.DB.GetContext(ctx, &fetchedProject, getProjectsQuery, id)
	})

	if errors.Is(err, sql.ErrNoRows) {
		return model.Project{}, project.ProjectDoesntExist
	} else if err != nil && fmt.Sprintf("%s", err.Error()[0:38]) == "pq: invalid input syntax for type uuid" {
		// TODO: this uuid syntax is a error defined in db, not in library
		// need to look into better ways to implement this
		return model.Project{}, project.InvalidUUID
	} else if err != nil {
		return model.Project{}, fmt.Errorf("%w: %s", dbErr, err)
	}

	if err != nil {
		return model.Project{}, err
	}

	transformedProject, err := transformToProject(fetchedProject)
	if err != nil {
		return model.Project{}, err
	}

	return transformedProject, nil
}

func (s Store) CreateProject(ctx context.Context, projectToCreate model.Project) (model.Project, error) {
	marshaledMetadata, err := json.Marshal(projectToCreate.Metadata)
	if err != nil {
		return model.Project{}, fmt.Errorf("%w: %s", parseErr, err)
	}

	var newProject Project
	err = s.DB.WithTimeout(ctx, func(ctx context.Context) error {
		return s.DB.GetContext(ctx, &newProject, createProjectQuery, projectToCreate.Name, projectToCreate.Slug, projectToCreate.Organization.Id, marshaledMetadata)
	})

	if err != nil {
		return model.Project{}, fmt.Errorf("%w: %s", dbErr, err)
	}

	transformedOrg, err := transformToProject(newProject)
	if err != nil {
		return model.Project{}, fmt.Errorf("%w: %s", parseErr, err)
	}

	return transformedOrg, nil
}

func (s Store) ListProject(ctx context.Context) ([]model.Project, error) {
	var fetchedProjects []Project
	err := s.DB.WithTimeout(ctx, func(ctx context.Context) error {
		return s.DB.SelectContext(ctx, &fetchedProjects, listProjectQuery)
	})

	if errors.Is(err, sql.ErrNoRows) {
		return []model.Project{}, project.ProjectDoesntExist
	}

	if err != nil {
		return []model.Project{}, fmt.Errorf("%w: %s", dbErr, err)
	}

	var transformedProjects []model.Project

	for _, o := range fetchedProjects {
		transformedOrg, err := transformToProject(o)
		if err != nil {
			return []model.Project{}, fmt.Errorf("%w: %s", parseErr, err)
		}

		transformedProjects = append(transformedProjects, transformedOrg)
	}

	return transformedProjects, nil
}

func (s Store) UpdateProject(ctx context.Context, toUpdate model.Project) (model.Project, error) {
	var updatedProject Project

	marshaledMetadata, err := json.Marshal(toUpdate.Metadata)
	if err != nil {
		return model.Project{}, fmt.Errorf("%w: %s", parseErr, err)
	}

	err = s.DB.WithTimeout(ctx, func(ctx context.Context) error {
		return s.DB.GetContext(ctx, &updatedProject, updateProjectQuery, toUpdate.Id, toUpdate.Name, toUpdate.Slug, toUpdate.Organization.Id, marshaledMetadata)
	})

	if errors.Is(err, sql.ErrNoRows) {
		return model.Project{}, project.ProjectDoesntExist
	} else if err != nil && fmt.Sprintf("%s", err.Error()[0:38]) == "pq: invalid input syntax for type uuid" {
		// TODO: this uuid syntax is a error defined in db, not in library
		// need to look into better ways to implement this
		return model.Project{}, fmt.Errorf("%w: %s", project.InvalidUUID, err)
	} else if err != nil {
		return model.Project{}, fmt.Errorf("%w: %s", dbErr, err)
	}

	toUpdate, err = transformToProject(updatedProject)
	if err != nil {
		return model.Project{}, fmt.Errorf("%s: %w", parseErr, err)
	}

	return toUpdate, nil
}

func transformToProject(from Project) (model.Project, error) {
	var unmarshalledMetadata map[string]string
	if err := json.Unmarshal(from.Metadata, &unmarshalledMetadata); err != nil {
		return model.Project{}, err
	}

	return model.Project{
		Id:           from.Id,
		Name:         from.Name,
		Slug:         from.Slug,
		Organization: model.Organization{Id: from.OrgId},
		Metadata:     unmarshalledMetadata,
		CreatedAt:    from.CreatedAt,
		UpdatedAt:    from.UpdatedAt,
	}, nil
}
