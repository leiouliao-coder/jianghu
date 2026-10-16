extends WsClient
## Test double: records requests instead of sending them.

var sent: Array[Dictionary] = []
var _n: int = 0


func is_open() -> bool:
	return true


func request(type: String, payload: Variant = null) -> String:
	_n += 1
	var id: String = "fake-%d" % _n
	sent.append({"id": id, "type": type, "payload": payload})
	return id


func last(type: String) -> Dictionary:
	for i: int in range(sent.size() - 1, -1, -1):
		if sent[i]["type"] == type:
			return sent[i]
	return {}
