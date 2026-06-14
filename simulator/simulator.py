#!/usr/bin/env python3
"""
古代筒车遥测数据模拟器
模拟10座筒车每5分钟通过4G DTU上报遥测数据
"""

import json
import math
import random
import time
from datetime import datetime
from urllib import request, error

API_BASE = "http://localhost:8080/api"
INTERVAL_SECONDS = 300

WATERWHEEL_CONFIGS = [
    {"id": 1,  "name": "筒车一号", "base_rpm": 2.8, "base_drop": 2.1, "base_flow": 1.8, "capacity": 120.0},
    {"id": 2,  "name": "筒车二号", "base_rpm": 3.2, "base_drop": 1.8, "base_flow": 1.5, "capacity": 95.0},
    {"id": 3,  "name": "筒车三号", "base_rpm": 2.5, "base_drop": 1.6, "base_flow": 1.4, "capacity": 85.0},
    {"id": 4,  "name": "筒车四号", "base_rpm": 3.5, "base_drop": 2.5, "base_flow": 2.1, "capacity": 140.0},
    {"id": 5,  "name": "筒车五号", "base_rpm": 3.0, "base_drop": 2.0, "base_flow": 1.7, "capacity": 110.0},
    {"id": 6,  "name": "筒车六号", "base_rpm": 2.2, "base_drop": 1.5, "base_flow": 1.2, "capacity": 75.0},
    {"id": 7,  "name": "筒车七号", "base_rpm": 3.1, "base_drop": 2.2, "base_flow": 1.9, "capacity": 115.0},
    {"id": 8,  "name": "筒车八号", "base_rpm": 2.9, "base_drop": 1.9, "base_flow": 1.6, "capacity": 100.0},
    {"id": 9,  "name": "筒车九号", "base_rpm": 3.3, "base_drop": 2.3, "base_flow": 2.0, "capacity": 130.0},
    {"id": 10, "name": "筒车十号", "base_rpm": 2.6, "base_drop": 1.7, "base_flow": 1.3, "capacity": 90.0},
]


def generate_telemetry(wheel_cfg, tick):
    phase = tick / 12.0

    rpm_factor = 1 + 0.15 * math.sin(phase) + random.gauss(0, 0.05)
    drop_factor = 1 + 0.1 * math.sin(phase + 0.5) + random.gauss(0, 0.03)
    flow_factor = 1 + 0.12 * math.sin(phase + 1.0) + random.gauss(0, 0.04)

    rotation_speed = max(0.5, wheel_cfg["base_rpm"] * rpm_factor)
    water_level_drop = max(0.5, wheel_cfg["base_drop"] * drop_factor)
    flow_velocity = max(0.3, wheel_cfg["base_flow"] * flow_factor)

    bucket_count = 24 if wheel_cfg["id"] in [1, 4, 7, 9] else (20 if wheel_cfg["id"] in [2, 5, 8] else 18)
    bucket_capacity = 0.08 if wheel_cfg["id"] in [1, 4, 7, 9] else (0.06 if wheel_cfg["id"] in [2, 5, 8, 10] else 0.05)
    diameter = 8.5 if wheel_cfg["id"] in [1, 4, 7, 9] else (7.5 if wheel_cfg["id"] in [2, 5, 8] else 6.8)

    fill_efficiency = 0.35 + 0.1 * math.sin(phase + 0.3)
    volume_per_rotation = bucket_count * fill_efficiency * bucket_capacity
    water_lift = min(volume_per_rotation * rotation_speed * 60.0, wheel_cfg["capacity"])

    low_eff_chance = 0.0
    if tick % 100 == wheel_cfg["id"]:
        low_eff_chance = 1.0

    if random.random() < low_eff_chance:
        rotation_speed *= 0.5
        water_lift *= 0.4

    return {
        "waterwheel_id": wheel_cfg["id"],
        "rotation_speed": round(rotation_speed, 3),
        "water_lift": round(water_lift, 2),
        "water_level_drop": round(water_level_drop, 3),
        "flow_velocity": round(flow_velocity, 3),
        "time": datetime.utcnow().isoformat() + "Z"
    }


def send_telemetry(data):
    payload = json.dumps(data).encode("utf-8")
    req = request.Request(
        f"{API_BASE}/telemetry",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST"
    )
    try:
        with request.urlopen(req, timeout=10) as resp:
            body = json.loads(resp.read().decode("utf-8"))
            return True, body
    except error.URLError as e:
        return False, str(e)
    except error.HTTPError as e:
        return False, f"HTTP {e.code}: {e.read().decode('utf-8')}"


def seed_historical_data():
    print("正在注入历史数据...")
    now = time.time()
    for hours_ago in range(24, 0, -1):
        tick = int((now - hours_ago * 3600) / INTERVAL_SECONDS)
        for wheel in WATERWHEEL_CONFIGS:
            data = generate_telemetry(wheel, tick)
            data["time"] = datetime.utcfromtimestamp(now - hours_ago * 3600).isoformat() + "Z"
            ok, resp = send_telemetry(data)
            if not ok:
                print(f"  注入失败 {wheel['name']}: {resp}")
    print("历史数据注入完成")


def run_continuous():
    print(f"筒车遥测模拟器启动，上报间隔 {INTERVAL_SECONDS} 秒")
    print(f"目标API: {API_BASE}")
    print("=" * 60)

    tick = int(time.time() / INTERVAL_SECONDS)

    while True:
        print(f"\n[{datetime.now().strftime('%Y-%m-%d %H:%M:%S')}] 上报周期 #{tick}")

        for wheel in WATERWHEEL_CONFIGS:
            data = generate_telemetry(wheel, tick)
            ok, resp = send_telemetry(data)
            status = "✓" if ok else "✗"
            mech = resp.get("mechanical_efficiency") if ok and isinstance(resp, dict) else None
            hyd = resp.get("hydraulic_efficiency") if ok and isinstance(resp, dict) else None
            eff_str = ""
            if mech is not None and hyd is not None:
                eff_str = f" [效率={mech*hyd*100:.1f}%]"
            print(f"  {status} {wheel['name']}: rpm={data['rotation_speed']:.2f} lift={data['water_lift']:.1f}m³/h{eff_str}")

        tick += 1
        next_time = tick * INTERVAL_SECONDS
        sleep_secs = max(1, next_time - time.time())
        print(f"  下次上报: {sleep_secs:.0f} 秒后")
        time.sleep(sleep_secs)


if __name__ == "__main__":
    import sys

    if "--seed" in sys.argv:
        seed_historical_data()

    try:
        run_continuous()
    except KeyboardInterrupt:
        print("\n模拟器已停止")
