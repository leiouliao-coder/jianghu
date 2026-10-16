class_name WsClient
extends Node
## WebSocket connection for protocol v1. Polls in _process, matches replies
## to requests by req_id and reconnects with backoff after recoverable drops.

signal opened
signal closed(code: int, reason: String)
signal reply_received(req_id: String, envelope: Dictionary)
signal pushed(envelope: Dictionary)
signal protocol_error(message: String)

var auto_reconnect: bool = true
var _peer: WebSocketPeer = WebSocketPeer.new()
var _url: String = ""
var _was_open: bool = false
var _closing_by_user: bool = false
var _next_id: int = 0
var _last_seq: int = 0
var _attempt: int = 0
var _reconnect_at: float = -1.0
var _session_prefix: String = ""


func _init() -> void:
	# Distinct per launch so a replayed req_id never collides across restarts.
	_session_prefix = "%x" % (Time.get_unix_time_from_system() * 1000.0 + randi() % 1000)


func connect_to(url: String) -> Error:
	if not (url.begins_with("ws://") or url.begins_with("wss://")):
		return ERR_INVALID_PARAMETER
	_url = url
	_closing_by_user = false
	_attempt = 0
	return _open()


func _open() -> Error:
	_peer = WebSocketPeer.new()
	_was_open = false
	_last_seq = 0
	_reconnect_at = -1.0
	return _peer.connect_to_url(_url)


func is_open() -> bool:
	return _peer.get_ready_state() == WebSocketPeer.STATE_OPEN


## Sends a request and returns its req_id, or "" if not connected.
func request(type: String, payload: Variant = null) -> String:
	if not is_open():
		return ""
	_next_id += 1
	var req_id: String = "%s-%d" % [_session_prefix, _next_id]
	_peer.send_text(Protocol.encode(type, req_id, payload))
	return req_id


func close_connection() -> void:
	_closing_by_user = true
	_reconnect_at = -1.0
	if _peer.get_ready_state() != WebSocketPeer.STATE_CLOSED:
		_peer.close(1000, "client closing")


func _process(_delta: float) -> void:
	if _reconnect_at > 0.0 and _now() >= _reconnect_at:
		_open()
	_peer.poll()
	var state: WebSocketPeer.State = _peer.get_ready_state()
	if state == WebSocketPeer.STATE_OPEN and not _was_open:
		_was_open = true
		_attempt = 0
		opened.emit()
	# Drain in every state: the last messages (e.g. "kicked") arrive right
	# before the close and must not be dropped.
	while _peer.get_available_packet_count() > 0:
		_handle(_peer.get_packet().get_string_from_utf8())
	if state == WebSocketPeer.STATE_CLOSED and _url != "" and _reconnect_at < 0.0:
		var code: int = _peer.get_close_code()
		var reason: String = _peer.get_close_reason()
		var had_session: bool = _was_open
		_was_open = false
		if had_session or _attempt == 0:
			closed.emit(code, reason)
		if auto_reconnect and not _closing_by_user and not Protocol.is_final_close(code):
			_reconnect_at = _now() + Protocol.backoff_seconds(_attempt)
			_attempt += 1
		else:
			_url = ""


func _handle(text: String) -> void:
	var envelope: Dictionary = Protocol.decode(text)
	if envelope.is_empty():
		protocol_error.emit("invalid server message")
		return
	if int(envelope["seq"]) <= _last_seq:
		protocol_error.emit("server seq went backwards")
	_last_seq = int(envelope["seq"])
	var req_id: String = String(envelope.get("req_id", ""))
	if req_id.is_empty():
		pushed.emit(envelope)
	else:
		reply_received.emit(req_id, envelope)


func _now() -> float:
	return Time.get_ticks_msec() / 1000.0
