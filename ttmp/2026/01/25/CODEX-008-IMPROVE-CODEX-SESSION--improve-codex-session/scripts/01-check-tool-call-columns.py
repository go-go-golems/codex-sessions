import sqlite3

path = "/home/manuel/.codex/sessions/session_index.sqlite"
con = sqlite3.connect(path)
cur = con.execute("PRAGMA user_version;")
print("user_version", cur.fetchone()[0])
cur = con.execute("PRAGMA table_info(tool_calls);")
print([row[1] for row in cur.fetchall()])
con.close()
