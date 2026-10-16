class_name CameraRig
extends Node3D
## Top-down oblique camera (baseline §3.1): wheel zooms within limits,
## middle-drag rotates, camera_reset restores the default angle.

const MIN_DISTANCE: float = 12.0
const MAX_DISTANCE: float = 60.0
const DEFAULT_DISTANCE: float = 28.0
const DEFAULT_YAW: float = 0.0
const PITCH_DEGREES: float = 55.0
const ZOOM_STEP: float = 2.0
const ROTATE_SPEED: float = 0.01  # radians per pixel

var distance: float = DEFAULT_DISTANCE
var yaw: float = DEFAULT_YAW
var _dragging: bool = false
@onready var camera: Camera3D = $Camera3D


func follow(target: Vector3) -> void:
	position = target
	_apply()


func zoom(steps: float) -> void:
	distance = clampf(distance + steps * ZOOM_STEP, MIN_DISTANCE, MAX_DISTANCE)
	_apply()


func rotate_by(pixels: float) -> void:
	yaw = wrapf(yaw - pixels * ROTATE_SPEED, -PI, PI)
	_apply()


func reset_view() -> void:
	distance = DEFAULT_DISTANCE
	yaw = DEFAULT_YAW
	_apply()


func _ready() -> void:
	_apply()


func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventMouseButton:
		var mb: InputEventMouseButton = event
		if mb.button_index == MOUSE_BUTTON_WHEEL_UP and mb.pressed:
			zoom(-1.0)
		elif mb.button_index == MOUSE_BUTTON_WHEEL_DOWN and mb.pressed:
			zoom(1.0)
		elif mb.button_index == MOUSE_BUTTON_MIDDLE:
			_dragging = mb.pressed
	elif event is InputEventMouseMotion and _dragging:
		rotate_by((event as InputEventMouseMotion).relative.x)
	elif event.is_action_pressed("camera_reset"):
		reset_view()


func _apply() -> void:
	if camera == null:
		return
	var pitch: float = deg_to_rad(PITCH_DEGREES)
	var offset: Vector3 = Vector3(sin(yaw) * cos(pitch), sin(pitch), cos(yaw) * cos(pitch)) * distance
	# Local transform only: at yaw 0 the camera sits on +z looking at the rig.
	camera.position = offset
	camera.rotation = Vector3(-pitch, yaw, 0.0)
