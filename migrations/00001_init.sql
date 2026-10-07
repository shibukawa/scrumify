-- +goose Up
CREATE TABLE account (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL DEFAULT 'human' CHECK (kind IN ('human', 'ai')),
    display_name TEXT NOT NULL,
    email TEXT NOT NULL DEFAULT '',
    responsible_human_id TEXT REFERENCES account (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE project (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'Asia/Tokyo',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE project_member (
    project_id BIGINT NOT NULL REFERENCES project (id) ON DELETE CASCADE,
    account_id TEXT NOT NULL REFERENCES account (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, account_id)
);

-- Tree of organizational units. A team is a leaf; sos and sosos are added in
-- later phases without changing how tickets reference their unit.
CREATE TABLE unit (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES project (id) ON DELETE CASCADE,
    parent_id BIGINT REFERENCES unit (id),
    kind TEXT NOT NULL DEFAULT 'team' CHECK (kind IN ('team', 'sos', 'sosos')),
    name TEXT NOT NULL,
    valid_from DATE NOT NULL DEFAULT CURRENT_DATE,
    valid_to DATE
);
CREATE INDEX unit_project ON unit (project_id);

CREATE TABLE sprint (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    unit_id BIGINT NOT NULL REFERENCES unit (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    goal TEXT NOT NULL DEFAULT '',
    starts_on DATE,
    ends_on DATE,
    state TEXT NOT NULL DEFAULT 'planned' CHECK (state IN ('planned', 'active', 'closed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX sprint_unit ON sprint (unit_id);

-- One entity for every work item. The row is the current state; ticket_event
-- records how it got there.
CREATE TABLE ticket (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES project (id) ON DELETE CASCADE,
    unit_id BIGINT NOT NULL REFERENCES unit (id),
    type TEXT NOT NULL CHECK (type IN ('story', 'pbi', 'task')),
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    acceptance_criteria TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    sprint_id BIGINT REFERENCES sprint (id) ON DELETE SET NULL,
    assignee_id TEXT REFERENCES account (id),
    rank DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL REFERENCES account (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX ticket_unit ON ticket (unit_id, type) WHERE deleted_at IS NULL;
CREATE INDEX ticket_sprint ON ticket (sprint_id) WHERE deleted_at IS NULL;

-- Directed links between tickets. parent_child goes parent -> child and keeps
-- its validity period so a re-parented ticket still shows where it used to be.
CREATE TABLE ticket_link (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('parent_child', 'dependency', 'block')),
    from_id BIGINT NOT NULL REFERENCES ticket (id) ON DELETE CASCADE,
    to_id BIGINT NOT NULL REFERENCES ticket (id) ON DELETE CASCADE,
    valid_from TIMESTAMPTZ NOT NULL DEFAULT now(),
    valid_to TIMESTAMPTZ
);
CREATE UNIQUE INDEX ticket_link_one_parent ON ticket_link (to_id)
    WHERE kind = 'parent_child' AND valid_to IS NULL;
CREATE INDEX ticket_link_children ON ticket_link (from_id)
    WHERE kind = 'parent_child' AND valid_to IS NULL;

-- Change history. Not an audit trail: field edits by one actor on one ticket
-- within one local day share a row that is updated in place. Transitions
-- (status, sprint) keep one row each so their timing survives.
CREATE TABLE ticket_event (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES ticket (id) ON DELETE CASCADE,
    actor_id TEXT NOT NULL REFERENCES account (id),
    kind TEXT NOT NULL CHECK (kind IN ('created', 'edited', 'status', 'sprint', 'deleted', 'restored')),
    local_day DATE NOT NULL,
    changes TEXT NOT NULL DEFAULT '{}',
    schema_version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ticket_event_ticket ON ticket_event (ticket_id, id);
CREATE UNIQUE INDEX ticket_event_daily_edit ON ticket_event (ticket_id, actor_id, local_day)
    WHERE kind = 'edited';

-- +goose Down
DROP TABLE ticket_event;
DROP TABLE ticket_link;
DROP TABLE ticket;
DROP TABLE sprint;
DROP TABLE unit;
DROP TABLE project_member;
DROP TABLE project;
DROP TABLE account;
