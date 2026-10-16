class_name SnapshotBuffer
extends RefCounted
## Buffers server snapshots and interpolates positions slightly in the
## past, so 10 Hz updates render smoothly. Never extrapolates: the client
## shows only positions the server actually reported.

const MAX_SNAPSHOTS: int = 32

var delay_ms: float = 100.0
var _snaps: Array[Dictionary] = []  # {t: server ms, entities: {id: Dictionary}}
var _offset_ms: float = 0.0  # server time minus local time
var _has_offset: bool = false


func push(server_time_ms: float, entities: Array, local_ms: float) -> void:
	if not _snaps.is_empty() and server_time_ms <= float(_snaps[-1]["t"]):
		return  # out of order or duplicate: ignore entirely
	var offset: float = server_time_ms - local_ms
	# Smooth the clock estimate so network jitter does not shake rendering.
	_offset_ms = offset if not _has_offset else lerpf(_offset_ms, offset, 0.1)
	_has_offset = true
	var by_id: Dictionary = {}
	for e: Variant in entities:
		if typeof(e) == TYPE_DICTIONARY and e.has("id"):
			by_id[int(e["id"])] = e
	_snaps.append({"t": server_time_ms, "entities": by_id})
	while _snaps.size() > MAX_SNAPSHOTS:
		_snaps.pop_front()


## Returns {id: {"pos": Vector2(x, z), "data": latest entity Dictionary}}
## for entities in the newest snapshot.
func sample(local_ms: float) -> Dictionary:
	var out: Dictionary = {}
	if _snaps.is_empty():
		return out
	var t: float = local_ms + _offset_ms - delay_ms
	var newest: Dictionary = _snaps[-1]
	var a: Dictionary = newest
	var b: Dictionary = newest
	for i: int in range(_snaps.size() - 1):
		if float(_snaps[i]["t"]) <= t and t < float(_snaps[i + 1]["t"]):
			a = _snaps[i]
			b = _snaps[i + 1]
			break
	if t < float(_snaps[0]["t"]):
		a = _snaps[0]
		b = _snaps[0]
	var span: float = float(b["t"]) - float(a["t"])
	var f: float = 0.0 if span <= 0.0 else clampf((t - float(a["t"])) / span, 0.0, 1.0)
	for id: int in newest["entities"]:
		var latest: Dictionary = newest["entities"][id]
		var pb: Vector2 = _pos(b["entities"].get(id, latest))
		var pa: Vector2 = _pos(a["entities"].get(id, b["entities"].get(id, latest)))
		out[id] = {"pos": pa.lerp(pb, f), "data": latest}
	return out


static func _pos(e: Dictionary) -> Vector2:
	return Vector2(float(e.get("x", 0.0)), float(e.get("z", 0.0)))
