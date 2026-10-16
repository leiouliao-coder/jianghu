class_name Protocol
extends RefCounted
## Protocol v1 envelope helpers; see shared/protocol/v1.md.

const VERSION: int = 1
const CLOSE_KICKED: int = 4001
const CLOSE_TOO_MANY_FAILURES: int = 4002
const CLOSE_BAD_VERSION: int = 4003
## Close codes after which reconnecting would only repeat the failure.
const FINAL_CLOSE_CODES: Array[int] = [CLOSE_KICKED, CLOSE_TOO_MANY_FAILURES, CLOSE_BAD_VERSION]


static func encode(type: String, req_id: String, payload: Variant = null) -> String:
	var envelope: Dictionary = {"v": VERSION, "type": type, "req_id": req_id}
	if payload != null:
		envelope["payload"] = payload
	return JSON.stringify(envelope)


## Parses a server message. Returns an empty Dictionary when the text is not
## a valid v1 server envelope.
static func decode(text: String) -> Dictionary:
	# A JSON instance reports errors through its return value instead of
	# printing them, so a bad server message is not logged as an engine error.
	var json: JSON = JSON.new()
	if json.parse(text) != OK or typeof(json.data) != TYPE_DICTIONARY:
		return {}
	var envelope: Dictionary = json.data
	if int(envelope.get("v", 0)) != VERSION:
		return {}
	if typeof(envelope.get("type")) != TYPE_STRING or String(envelope["type"]).is_empty():
		return {}
	if typeof(envelope.get("seq")) != TYPE_FLOAT or float(envelope["seq"]) < 1.0:
		return {}
	var payload: Variant = envelope.get("payload", {})
	if payload == null:
		payload = {}
	if typeof(payload) != TYPE_DICTIONARY:
		return {}
	envelope["payload"] = payload
	envelope["seq"] = int(envelope["seq"])
	return envelope


## Reconnect delay in seconds: 0.5 doubling per attempt, capped at 30.
static func backoff_seconds(attempt: int) -> float:
	return minf(30.0, 0.5 * pow(2.0, float(maxi(attempt, 0))))


static func is_final_close(code: int) -> bool:
	return FINAL_CLOSE_CODES.has(code)
