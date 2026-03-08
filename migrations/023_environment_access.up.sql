CREATE TABLE project_environment_access (
    project_id     UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    role_name      TEXT NOT NULL REFERENCES roles(name) ON UPDATE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    PRIMARY KEY (project_id, role_name, environment_id)
);
