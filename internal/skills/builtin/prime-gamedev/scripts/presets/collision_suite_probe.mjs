function overlapsBox(a, b, eps = 1e-4) {
  const overlapX = a.minX < b.maxX - eps && a.maxX > b.minX + eps
  const overlapY = a.minY < b.maxY - eps && a.maxY > b.minY + eps
  if (a.minZ === undefined || b.minZ === undefined) {
    return overlapX && overlapY
  }
  const overlapZ = a.minZ < b.maxZ - eps && a.maxZ > b.minZ + eps
  return overlapX && overlapY && overlapZ
}

function boxFromCenter(center, halfExtents) {
  const box = {
    minX: center.x - halfExtents.x,
    maxX: center.x + halfExtents.x,
    minY: center.y - halfExtents.y,
    maxY: center.y + halfExtents.y
  }
  if (center.z !== undefined && halfExtents.z !== undefined) {
    box.minZ = center.z - halfExtents.z
    box.maxZ = center.z + halfExtents.z
  }
  return box
}

export function verifyCodeCollisions({
  objects = [],
  stepMove,
  actorHalfExtents,
  boundsTolerance = 0.05,
  is3D = false
}) {
  const failures = []
  const directions = is3D
    ? [
        { name: '+X', dx: 1, dy: 0, dz: 0 },
        { name: '-X', dx: -1, dy: 0, dz: 0 },
        { name: '+Y', dx: 0, dy: 1, dz: 0 },
        { name: '-Y', dx: 0, dy: -1, dz: 0 },
        { name: '+Z', dx: 0, dy: 0, dz: 1 },
        { name: '-Z', dx: 0, dy: 0, dz: -1 }
      ]
    : [
        { name: '+X', dx: 1, dy: 0 },
        { name: '-X', dx: -1, dy: 0 },
        { name: '+Y', dx: 0, dy: 1 },
        { name: '-Y', dx: 0, dy: -1 }
      ]

  for (const obj of objects) {
    const id = String(obj.id || obj.name || 'unnamed-object')
    if (!obj.collider) {
      failures.push({ id, reason: 'missing_collider' })
      continue
    }

    if (obj.visualBounds) {
      for (const axis of is3D ? ['minX', 'maxX', 'minY', 'maxY', 'minZ', 'maxZ'] : ['minX', 'maxX', 'minY', 'maxY']) {
        const diff = Math.abs(Number(obj.collider[axis]) - Number(obj.visualBounds[axis]))
        if (!Number.isFinite(diff) || diff > boundsTolerance) {
          failures.push({ id, reason: 'visual_collider_mismatch', axis, diff })
        }
      }
    }

    if (typeof stepMove !== 'function' || !actorHalfExtents) {
      continue
    }

    const cx = (obj.collider.minX + obj.collider.maxX) / 2
    const cy = (obj.collider.minY + obj.collider.maxY) / 2
    const cz = is3D ? (obj.collider.minZ + obj.collider.maxZ) / 2 : undefined

    for (const dir of directions) {
      const margin = 0.1
      const start = {
        x: dir.dx === 0 ? cx : dir.dx > 0 ? obj.collider.minX - actorHalfExtents.x - margin : obj.collider.maxX + actorHalfExtents.x + margin,
        y: dir.dy === 0 ? cy : dir.dy > 0 ? obj.collider.minY - actorHalfExtents.y - margin : obj.collider.maxY + actorHalfExtents.y + margin
      }
      const delta = {
        x: dir.dx * (actorHalfExtents.x * 2 + margin * 3),
        y: dir.dy * (actorHalfExtents.y * 2 + margin * 3)
      }
      if (is3D) {
        start.z = dir.dz === 0 ? cz : dir.dz > 0 ? obj.collider.minZ - actorHalfExtents.z - margin : obj.collider.maxZ + actorHalfExtents.z + margin
        delta.z = dir.dz * (actorHalfExtents.z * 2 + margin * 3)
      }

      const next = stepMove(start, delta, obj)
      const coords = is3D ? [next?.x, next?.y, next?.z] : [next?.x, next?.y]
      if (coords.some((v) => !Number.isFinite(Number(v)))) {
        failures.push({ id, side: dir.name, reason: 'nan_after_collision' })
        continue
      }
      const actorBox = boxFromCenter(next, actorHalfExtents)
      if (overlapsBox(actorBox, obj.collider)) {
        failures.push({ id, side: dir.name, reason: 'penetrated_collider', next })
      }
    }
  }

  return {
    ok: failures.length === 0,
    checkedObjects: objects.length,
    sidesPerObject: directions.length,
    failures
  }
}
