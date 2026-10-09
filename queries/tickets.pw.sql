package queries

type Ticket {
  id: int
  projectId: int
  unitId: int
  type: string
  title: string
  description: string
  acceptanceCriteria: string
  status: string
  sprintId: int?
  assigneeId: string?
  assigneeName: string
  rank: float
  parentId: int?
  createdAt: datetime
  updatedAt: datetime
  deleted: bool
}

type NewTicketId { id: int }

type RankValue { value: float }

type TicketEvent {
  id: int
  ticketId: int
  actorId: string
  actorName: string
  kind: string
  localDay: date
  changes: string
  createdAt: datetime
  updatedAt: datetime
}

statement TicketRows(): sql.relation<Ticket> {
  SELECT
    t.id,
    t.project_id AS projectId,
    t.unit_id AS unitId,
    t.type,
    t.title,
    t.description,
    t.acceptance_criteria AS acceptanceCriteria,
    t.status,
    t.sprint_id AS sprintId,
    t.assignee_id AS assigneeId,
    COALESCE(a.display_name, '') AS assigneeName,
    t.rank,
    l.from_id AS parentId,
    t.created_at AS createdAt,
    t.updated_at AS updatedAt,
    t.deleted_at IS NOT NULL AS deleted
  FROM ticket t
  LEFT JOIN account a
    ON a.id = t.assignee_id
  LEFT JOIN ticket_link l
    ON l.to_id = t.id AND l.kind = 'parent_child' AND l.valid_to IS NULL
}

export statement GetTicket(id: int): sql.optional<Ticket> {
  SELECT
    r.id,
    r.projectId,
    r.unitId,
    r.type,
    r.title,
    r.description,
    r.acceptanceCriteria,
    r.status,
    r.sprintId,
    r.assigneeId,
    r.assigneeName,
    r.rank,
    r.parentId,
    r.createdAt,
    r.updatedAt,
    r.deleted
  FROM subquery TicketRows() AS r
  WHERE r.id = {id}
}

export statement ListUnitTickets(unitId: int): sql.many<Ticket> {
  SELECT
    r.id,
    r.projectId,
    r.unitId,
    r.type,
    r.title,
    r.description,
    r.acceptanceCriteria,
    r.status,
    r.sprintId,
    r.assigneeId,
    r.assigneeName,
    r.rank,
    r.parentId,
    r.createdAt,
    r.updatedAt,
    r.deleted
  FROM subquery TicketRows() AS r
  WHERE r.unitId = {unitId} AND NOT r.deleted
  ORDER BY r.rank, r.id
}

export statement MaxRank(unitId: int): sql.one<RankValue> {
  SELECT COALESCE(MAX(rank), 0) AS value
  FROM ticket
  WHERE unit_id = {unitId}
}

export statement InsertTicket(projectId: int, unitId: int, type: string, title: string, status: string, sprintId: int?, rank: float, createdBy: string): sql.one<NewTicketId> {
  INSERT INTO ticket (project_id, unit_id, type, title, status, sprint_id, rank, created_by)
  VALUES ({projectId}, {unitId}, {type}, {title}, {status}, {sprintId}, {rank}, {createdBy})
  RETURNING id
}

export statement SetTicketTitle(id: int, value: string): sql.exec {
  UPDATE ticket
  SET title = {value}, updated_at = now()
  WHERE id = {id}
}

export statement SetTicketDescription(id: int, value: string): sql.exec {
  UPDATE ticket
  SET description = {value}, updated_at = now()
  WHERE id = {id}
}

export statement SetTicketAcceptanceCriteria(id: int, value: string): sql.exec {
  UPDATE ticket
  SET acceptance_criteria = {value}, updated_at = now()
  WHERE id = {id}
}

export statement SetTicketStatus(id: int, value: string): sql.exec {
  UPDATE ticket
  SET status = {value}, updated_at = now()
  WHERE id = {id}
}

export statement SetTicketSprint(id: int, value: int?): sql.exec {
  UPDATE ticket
  SET sprint_id = {value}, updated_at = now()
  WHERE id = {id}
}

export statement SetTicketAssignee(id: int, value: string?): sql.exec {
  UPDATE ticket
  SET assignee_id = {value}, updated_at = now()
  WHERE id = {id}
}

export statement SetTicketRank(id: int, value: float): sql.exec {
  UPDATE ticket
  SET rank = {value}, updated_at = now()
  WHERE id = {id}
}

export statement SetTicketDeleted(id: int, deleted: bool): sql.exec {
  UPDATE ticket
  SET deleted_at = CASE WHEN {deleted} THEN now() ELSE NULL END, updated_at = now()
  WHERE id = {id}
}

export statement CloseParentLink(childId: int): sql.exec {
  UPDATE ticket_link
  SET valid_to = now()
  WHERE to_id = {childId} AND kind = 'parent_child' AND valid_to IS NULL
}

export statement InsertParentLink(parentId: int, childId: int): sql.exec {
  INSERT INTO ticket_link (kind, from_id, to_id)
  VALUES ('parent_child', {parentId}, {childId})
}

export statement InsertTicketEvent(ticketId: int, actorId: string, kind: string, localDay: date, changes: string): sql.exec {
  INSERT INTO ticket_event (ticket_id, actor_id, kind, local_day, changes)
  VALUES ({ticketId}, {actorId}, {kind}, {localDay}, {changes})
}

export statement GetDailyEdit(ticketId: int, actorId: string, localDay: date): sql.optional<TicketEvent> {
  SELECT
    e.id,
    e.ticket_id AS ticketId,
    e.actor_id AS actorId,
    a.display_name AS actorName,
    e.kind,
    e.local_day AS localDay,
    e.changes,
    e.created_at AS createdAt,
    e.updated_at AS updatedAt
  FROM ticket_event e
  JOIN account a
    ON a.id = e.actor_id
  WHERE
    e.ticket_id = {ticketId}
    AND e.actor_id = {actorId}
    AND e.local_day = {localDay}
    AND e.kind = 'edited'
}

export statement UpdateTicketEventChanges(id: int, changes: string): sql.exec {
  UPDATE ticket_event
  SET changes = {changes}, updated_at = now()
  WHERE id = {id}
}

export statement DeleteTicketEvent(id: int): sql.exec {
  DELETE FROM ticket_event
  WHERE id = {id}
}

export statement ListTicketEvents(ticketId: int): sql.many<TicketEvent> {
  SELECT
    e.id,
    e.ticket_id AS ticketId,
    e.actor_id AS actorId,
    a.display_name AS actorName,
    e.kind,
    e.local_day AS localDay,
    e.changes,
    e.created_at AS createdAt,
    e.updated_at AS updatedAt
  FROM ticket_event e
  JOIN account a
    ON a.id = e.actor_id
  WHERE e.ticket_id = {ticketId}
  ORDER BY e.updated_at DESC, e.id DESC
}

export statement LockTicket(id: int): sql.optional<NewTicketId> {
  SELECT id
  FROM ticket
  WHERE id = {id}
  FOR UPDATE
}
