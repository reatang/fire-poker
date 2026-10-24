"""Render the editable pixel-grid Fire Poker mark into SVG, PNG and Windows ICO.

Run from the repository root: uv run --with pillow tools/generate-brand-icon.py
All contours use integer grid coordinates; raster sizes use nearest-neighbor.
"""
from pathlib import Path
import io
import struct
from PIL import Image

ROOT = Path(__file__).resolve().parents[1]
LAYERS = [
    ('#71312F', [(13,1),(15,1),(15,4),(16,4),(16,6),(17,6),(17,8),(18,8),(18,10),(20,10),(20,13),(21,13),(21,18),(20,18),(20,20),(18,20),(18,22),(7,22),(7,21),(5,21),(5,19),(3,19),(3,13),(4,13),(4,9),(5,9),(5,6),(7,6),(7,8),(9,8),(9,6),(10,6),(10,4),(11,4),(11,3),(12,3),(12,2),(13,2)]),
    ('#ED4B2E', [(13,3),(14,3),(14,5),(15,5),(15,7),(16,7),(16,9),(17,9),(17,11),(19,11),(19,14),(20,14),(20,17),(19,17),(19,19),(17,19),(17,21),(8,21),(8,20),(6,20),(6,18),(4,18),(4,14),(5,14),(5,11),(6,11),(6,8),(7,8),(7,10),(10,10),(10,7),(11,7),(11,5),(12,5),(12,4),(13,4)]),
    ('#FF8B2B', [(13,5),(14,5),(14,8),(15,8),(15,11),(17,11),(17,13),(18,13),(18,18),(16,18),(16,20),(9,20),(9,19),(7,19),(7,17),(6,17),(6,13),(8,13),(8,14),(10,14),(10,11),(11,11),(11,8),(12,8),(12,6),(13,6)]),
    ('#FFD35A', [(13,10),(14,10),(14,13),(15,13),(15,15),(16,15),(16,19),(15,19),(15,20),(10,20),(10,19),(9,19),(9,16),(10,16),(10,14),(11,14),(11,13),(12,13),(12,11),(13,11)]),
    ('#FFF0AD', [(12,15),(13,15),(13,17),(14,17),(14,19),(11,19),(11,17),(12,17)]),
]

def inside(x, y, polygon):
    odd = False
    for a, b in zip(polygon, polygon[1:] + polygon[:1]):
        if (a[1] > y) != (b[1] > y) and x < (b[0]-a[0])*(y-a[1])/(b[1]-a[1])+a[0]:
            odd = not odd
    return odd

base = Image.new('RGBA', (24,24))
for color, polygon in LAYERS:
    rgba = tuple(bytes.fromhex(color[1:])) + (255,)
    for y in range(24):
        for x in range(24):
            if inside(x+.5, y+.5, polygon):
                base.putpixel((x,y), rgba)

brand = ROOT / 'frontend/public/brand'
brand.mkdir(parents=True, exist_ok=True)
svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" shape-rendering="crispEdges"><title>Fire Poker</title>'
for color, points in LAYERS:
    svg += f'<polygon fill="{color}" points="' + ' '.join(f'{x},{y}' for x,y in points) + '"/>'
svg += '</svg>\n'
(brand/'fire-poker.svg').write_text(svg, encoding='utf-8')
for size in (24,32,48,64,128,256,1024):
    base.resize((size,size), Image.Resampling.NEAREST).save(brand/f'fire-poker-{size}.png')
base.resize((1024,1024), Image.Resampling.NEAREST).save(ROOT/'build/appicon.png')
base.resize((1024,1024), Image.Resampling.NEAREST).save(ROOT/'build/appicon_dev.png')

# Explicit PNG frames avoid ICO encoders smoothing the pixel contours.
sizes = (16,24,32,48,64,128,256)
frames = []
for size in sizes:
    buf = io.BytesIO()
    base.resize((size,size),Image.Resampling.NEAREST).save(buf,format='PNG')
    frames.append(buf.getvalue())
ico = struct.pack('<HHH',0,1,len(sizes))
offset = 6 + 16*len(sizes)
for size, data in zip(sizes,frames):
    ico += struct.pack('<BBBBHHII',size%256,size%256,0,0,1,32,len(data),offset)
    offset += len(data)
ico += b''.join(frames)
for dest in ('build/windows/icon.ico','build/windows/icon_dev.ico','frontend/public/brand/fire-poker.ico'):
    (ROOT/dest).write_bytes(ico)

# Modern ICNS chunks hold PNG data, preserving the same nearest-neighbor grid.
chunks = []
for kind, size in ((b'icp4',16),(b'icp5',32),(b'icp6',64),(b'ic07',128),(b'ic08',256),(b'ic09',512),(b'ic10',1024)):
    buf = io.BytesIO()
    base.resize((size,size),Image.Resampling.NEAREST).save(buf,format='PNG')
    data = buf.getvalue()
    chunks.append(kind + struct.pack('>I',8+len(data)) + data)
icns = b'icns' + struct.pack('>I',8+sum(map(len,chunks))) + b''.join(chunks)
for dest in ('build/darwin/icons.icns','build/darwin/icons_dev.icns'):
    (ROOT/dest).write_bytes(icns)
print('Generated Fire Poker SVG, PNG sizes, pixel-sharp ICO and ICNS.')
