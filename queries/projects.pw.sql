package queries

type NewId { id: int }

type Project { id: int, name: string, timezone: string }

type Unit { id: int, projectId: int, parentId: int?, kind: string, name: string }

type Sprint {
  id: int
  unitId: int
  name: string
  goal: string
  startsOn: date?
  endsOn: date?
  state: string
}

export statement ListProjectsForAccount(accountId: string): sql.many<Project> {
  SELECT p.id, p.name, p.timezone
  FROM project p
  JOIN project_member m
    ON m.project_id = p.id
  WHERE m.account_id = {accountId}
  ORDER BY p.name, p.id
}

export statement GetProjectForMember(id: int, accountId: string): sql.optional<Project> {
  SELECT p.id, p.name, p.timezone
  FROM project p
  JOIN project_member m
    ON m.project_id = p.id
  WHERE p.id = {id} AND m.account_id = {accountId}
}

export statement InsertProject(name: string, timezone: string): sql.one<NewId> {
  INSERT INTO project (name, timezone)
  VALUES ({name}, {timezone})
  RETURNING id
}

export statement RenameProject(id: int, name: string): sql.exec {
  UPDATE project
  SET name = {name}
  WHERE id = {id}
}

export statement AddProjectMember(projectId: int, accountId: string): sql.exec {
  INSERT INTO project_member (project_id, account_id)
  VALUES ({projectId}, {accountId})
  ON CONFLICT DO NOTHING
}

export statement InsertUnit(projectId: int, parentId: int?, kind: string, name: string): sql.one<NewId> {
  INSERT INTO unit (project_id, parent_id, kind, name)
  VALUES ({projectId}, {parentId}, {kind}, {name})
  RETURNING id
}

export statement ListUnits(projectId: int): sql.many<Unit> {
  SELECT id, project_id AS projectId, parent_id AS parentId, kind, name
  FROM unit
  WHERE project_id = {projectId} AND valid_to IS NULL
  ORDER BY id
}

export statement GetUnit(id: int): sql.optional<Unit> {
  SELECT id, project_id AS projectId, parent_id AS parentId, kind, name
  FROM unit
  WHERE id = {id}
}

export statement RenameUnit(id: int, name: string): sql.exec {
  UPDATE unit
  SET name = {name}
  WHERE id = {id}
}

export statement InsertSprint(unitId: int, name: string): sql.one<NewId> {
  INSERT INTO sprint (unit_id, name)
  VALUES ({unitId}, {name})
  RETURNING id
}

export statement ListSprints(unitId: int): sql.many<Sprint> {
  SELECT id, unit_id AS unitId, name, goal, starts_on AS startsOn, ends_on AS endsOn, state
  FROM sprint
  WHERE unit_id = {unitId}
  ORDER BY id
}

export statement GetSprint(id: int): sql.optional<Sprint> {
  SELECT id, unit_id AS unitId, name, goal, starts_on AS startsOn, ends_on AS endsOn, state
  FROM sprint
  WHERE id = {id}
}

export statement UpdateSprint(id: int, name: string, goal: string, startsOn: date?, endsOn: date?, state: string): sql.exec {
  UPDATE sprint
  SET name = {name}, goal = {goal}, starts_on = {startsOn}, ends_on = {endsOn}, state = {state}
  WHERE id = {id}
}
