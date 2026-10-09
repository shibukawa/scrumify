package queries

type Account { id: string, kind: string, displayName: string, email: string }

export statement GetAccount(id: string): sql.optional<Account> {
  SELECT id, kind, display_name AS displayName, email
  FROM account
  WHERE id = {id}
}

export statement UpsertAccount(id: string, displayName: string, email: string): sql.exec {
  INSERT INTO account (id, display_name, email)
  VALUES ({id}, {displayName}, {email})
  ON CONFLICT (id) DO UPDATE SET display_name = EXCLUDED.display_name, email = EXCLUDED.email
}

export statement ListAccounts(): sql.many<Account> {
  SELECT id, kind, display_name AS displayName, email
  FROM account
  ORDER BY display_name, id
}

export statement ListProjectMembers(projectId: int): sql.many<Account> {
  SELECT a.id, a.kind, a.display_name AS displayName, a.email
  FROM project_member m
  JOIN account a
    ON a.id = m.account_id
  WHERE m.project_id = {projectId}
  ORDER BY a.display_name, a.id
}
