#!/usr/bin/env python3
"""Generate NovaRuo's globe texture from Natural Earth country boundaries.

The Three.js sphere expects a true 2:1 equirectangular texture.  Do not replace
this with a Web-Mercator or decorative world map: latitude markers will no
longer line up with the rendered coastlines.

Source data: Natural Earth, 1:110m Admin 0 Countries (public domain)
https://www.naturalearthdata.com/downloads/110m-cultural-vectors/
"""

from __future__ import annotations

import argparse
import json
import math
from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter


WIDTH = 2048
HEIGHT = 1024


def project(longitude: float, latitude: float) -> tuple[float, float]:
    """Map WGS84 longitude/latitude to a 2:1 equirectangular canvas."""
    return (
        (longitude + 180.0) / 360.0 * WIDTH,
        (90.0 - latitude) / 180.0 * HEIGHT,
    )


def unwrap_ring(ring: list[list[float]]) -> list[tuple[float, float]]:
    """Keep dateline-crossing polygons locally continuous before drawing."""
    if not ring:
        return []
    points: list[tuple[float, float]] = []
    previous = float(ring[0][0])
    points.append((previous, float(ring[0][1])))
    for raw_longitude, raw_latitude, *_ in ring[1:]:
        longitude = float(raw_longitude)
        while longitude - previous > 180.0:
            longitude -= 360.0
        while longitude - previous < -180.0:
            longitude += 360.0
        points.append((longitude, float(raw_latitude)))
        previous = longitude
    return points


def iter_polygons(geometry: dict) -> list[list[list[list[float]]]]:
    if geometry.get("type") == "Polygon":
        return [geometry.get("coordinates", [])]
    if geometry.get("type") == "MultiPolygon":
        return geometry.get("coordinates", [])
    return []


def draw_polygon_copies(
    draw: ImageDraw.ImageDraw,
    polygon: list[list[list[float]]],
    *,
    fill: tuple[int, int, int, int] | None,
    outline: tuple[int, int, int, int] | None,
    width: int = 1,
) -> None:
    if not polygon:
        return
    outer = unwrap_ring(polygon[0])
    for shift in (-360.0, 0.0, 360.0):
        projected = [project(longitude + shift, latitude) for longitude, latitude in outer]
        if fill is not None:
            draw.polygon(projected, fill=fill)
        if outline is not None:
            draw.line(projected, fill=outline, width=width, joint="curve")


def point_in_ring(longitude: float, latitude: float, ring: list[list[float]]) -> bool:
    inside = False
    points = unwrap_ring(ring)
    if len(points) < 3:
        return False
    test_longitude = longitude
    average = sum(point[0] for point in points) / len(points)
    while test_longitude - average > 180.0:
        test_longitude -= 360.0
    while test_longitude - average < -180.0:
        test_longitude += 360.0
    previous = points[-1]
    for current in points:
        x1, y1 = previous
        x2, y2 = current
        crosses = (y1 > latitude) != (y2 > latitude)
        if crosses:
            boundary = (x2 - x1) * (latitude - y1) / (y2 - y1) + x1
            if test_longitude < boundary:
                inside = not inside
        previous = current
    return inside


def point_in_geojson(longitude: float, latitude: float, features: list[dict]) -> bool:
    for feature in features:
        for polygon in iter_polygons(feature.get("geometry") or {}):
            if polygon and point_in_ring(longitude, latitude, polygon[0]):
                return True
    return False


def generate(source: Path, destination: Path) -> None:
    data = json.loads(source.read_text(encoding="utf-8"))
    features = data.get("features", [])

    # Regression controls: both operational markers must resolve to land in
    # the same WGS84 data used to create the visible texture.
    controls = {
        "Hong Kong": (114.1694, 22.3193),
        "Malaysia (Sepang)": (101.7094012, 2.8008619),
    }
    for name, (longitude, latitude) in controls.items():
        if not point_in_geojson(longitude, latitude, features):
            raise RuntimeError(f"{name} control point is not on generated land")

    base = Image.new("RGBA", (WIDTH, HEIGHT), (2, 13, 17, 255))

    # Subtle geographic grid. It is projected with the same transform and
    # makes incorrect texture replacements immediately visible in review.
    grid = Image.new("RGBA", base.size, (0, 0, 0, 0))
    grid_draw = ImageDraw.Draw(grid)
    for longitude in range(-180, 181, 15):
        x, _ = project(longitude, 0)
        grid_draw.line((x, 0, x, HEIGHT), fill=(24, 107, 107, 24), width=1)
    for latitude in range(-75, 76, 15):
        _, y = project(0, latitude)
        grid_draw.line((0, y, WIDTH, y), fill=(24, 107, 107, 20), width=1)
    base.alpha_composite(grid)

    land = Image.new("RGBA", base.size, (0, 0, 0, 0))
    land_draw = ImageDraw.Draw(land)
    glow = Image.new("RGBA", base.size, (0, 0, 0, 0))
    glow_draw = ImageDraw.Draw(glow)

    for feature in features:
        for polygon in iter_polygons(feature.get("geometry") or {}):
            draw_polygon_copies(
                land_draw,
                polygon,
                fill=(7, 49, 52, 255),
                outline=(59, 218, 205, 205),
                width=2,
            )
            draw_polygon_copies(
                glow_draw,
                polygon,
                fill=None,
                outline=(36, 240, 220, 150),
                width=3,
            )

    base.alpha_composite(glow.filter(ImageFilter.GaussianBlur(radius=7)))
    base.alpha_composite(land)

    # Deterministic vertex lights preserve the technological visual language
    # without moving or reshaping any coastline.
    lights = Image.new("RGBA", base.size, (0, 0, 0, 0))
    lights_draw = ImageDraw.Draw(lights)
    counter = 0
    for feature in features:
        for polygon in iter_polygons(feature.get("geometry") or {}):
            if not polygon:
                continue
            for longitude, latitude in unwrap_ring(polygon[0]):
                counter += 1
                if counter % 9:
                    continue
                x, y = project(longitude, latitude)
                for shifted_x in (x - WIDTH, x, x + WIDTH):
                    lights_draw.ellipse(
                        (shifted_x - 1.4, y - 1.4, shifted_x + 1.4, y + 1.4),
                        fill=(102, 255, 236, 185),
                    )
    base.alpha_composite(lights.filter(ImageFilter.GaussianBlur(radius=2)))
    base.alpha_composite(lights)

    destination.parent.mkdir(parents=True, exist_ok=True)
    base.convert("RGB").save(destination, format="PNG", optimize=True)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("source", type=Path, help="Natural Earth countries GeoJSON")
    parser.add_argument("destination", type=Path, help="Output PNG")
    args = parser.parse_args()
    generate(args.source, args.destination)


if __name__ == "__main__":
    main()
