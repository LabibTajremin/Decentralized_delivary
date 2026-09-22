"""Generate GoKlay demo imagery.

The images are generated rather than sourced so they are licence-clean, small,
deterministic, and reproducible: re-running this script yields byte-identical
files, so a regenerated asset never shows up as a spurious diff.

Palette and type follow the GoKlay design system.
"""
import json, math, os, hashlib
from PIL import Image, ImageDraw, ImageFont

ROOT = "internal/platform/assets/demo"
FONT = "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf"

# GoKlay design-system palette.
PRIMARY   = (0x00, 0xC8, 0x53)
PRIMARY_D = (0x16, 0xA3, 0x4A)
INK       = (0x0F, 0x17, 0x2B)
AMBER     = (0xF5, 0x9E, 0x0B)
RED       = (0xEF, 0x44, 0x44)
MIST      = (0xF8, 0xFA, 0xFC)

# Per-category accent, so a grid of demo merchants is legible at a glance
# rather than eight tiles of the same green.
CATEGORY_ACCENT = {
    "food":     (PRIMARY, PRIMARY_D),
    "grocery":  (AMBER, (0xD9, 0x77, 0x06)),
    "pharmacy": (RED, (0xDC, 0x26, 0x26)),
    "parcel":   ((0x38, 0x7C, 0xF7), (0x1D, 0x4E, 0xD8)),
}

def font(size):
    return ImageFont.truetype(FONT, size)

def lerp(a, b, t):
    return tuple(round(x + (y - x) * t) for x, y in zip(a, b))

def gradient(size, top, bottom, diagonal=True):
    """A linear gradient. Diagonal reads as depth without costing a photo."""
    w, h = size
    img = Image.new("RGB", size)
    px = img.load()
    span = (w + h) if diagonal else h
    for y in range(h):
        for x in range(w):
            t = ((x + y) if diagonal else y) / max(span - 1, 1)
            px[x, y] = lerp(top, bottom, t)
    return img

def centred(draw, box, text, fnt, fill):
    x0, y0, x1, y1 = box
    l, t, r, b = draw.textbbox((0, 0), text, font=fnt)
    draw.text((x0 + (x1 - x0 - (r - l)) / 2 - l,
               y0 + (y1 - y0 - (b - t)) / 2 - t), text, font=fnt, fill=fill)

def rounded(img, radius):
    """Round the corners by compositing onto a mask, so logos are drop-in."""
    mask = Image.new("L", img.size, 0)
    ImageDraw.Draw(mask).rounded_rectangle([0, 0, img.size[0] - 1, img.size[1] - 1],
                                           radius=radius, fill=255)
    out = Image.new("RGBA", img.size, (0, 0, 0, 0))
    out.paste(img, (0, 0), mask)
    return out

def initials(name):
    words = [w for w in name.replace("'", " ").split() if w[0].isalnum()]
    if len(words) == 1:
        return words[0][:2].upper()
    return (words[0][0] + words[1][0]).upper()

def logo(name, category, path, size=256):
    top, bottom = CATEGORY_ACCENT[category]
    img = gradient((size, size), top, bottom)
    d = ImageDraw.Draw(img)
    # A soft arc in the corner: enough shape that the tile is not a flat square.
    d.ellipse([size * 0.55, -size * 0.35, size * 1.45, size * 0.55],
              fill=lerp(top, MIST, 0.18))
    centred(d, (0, 0, size, size), initials(name), font(int(size * 0.42)), MIST)
    rounded(img, int(size * 0.22)).save(path, optimize=True)

def cover(name, category, path, size=(1200, 600)):
    top, bottom = CATEGORY_ACCENT[category]
    img = gradient(size, lerp(top, INK, 0.15), lerp(bottom, INK, 0.55))
    d = ImageDraw.Draw(img)
    w, h = size
    # Concentric arcs echoing a delivery radius — the product's core idea, and
    # a pattern that survives being cropped to any aspect ratio.
    for i in range(7):
        r = w * (0.18 + i * 0.11)
        d.ellipse([w * 0.80 - r, h * 0.55 - r, w * 0.80 + r, h * 0.55 + r],
                  outline=lerp(top, bottom, i / 6), width=2)
    d.text((64, h - 168), name, font=font(58), fill=MIST)
    d.text((64, h - 96), category.upper(), font=font(30), fill=lerp(top, MIST, 0.45))
    img.save(path, optimize=True)

def category_tile(slug, label, glyph, path, size=256):
    top, bottom = CATEGORY_ACCENT[slug]
    img = gradient((size, size), top, bottom)
    d = ImageDraw.Draw(img)
    centred(d, (0, 0, size, size * 0.82), glyph, font(int(size * 0.44)), MIST)
    centred(d, (0, size * 0.70, size, size), label.upper(), font(int(size * 0.11)),
            lerp(top, MIST, 0.7))
    rounded(img, int(size * 0.22)).save(path, optimize=True)

def avatar(name, path, size=192):
    """Deterministic hue per name, so a demo user keeps the same face."""
    h = int(hashlib.sha256(name.encode()).hexdigest()[:8], 16) % 360
    def hsv(hh, s, v):
        i = int(hh / 60) % 6
        f = hh / 60 - int(hh / 60)
        p, q, t = v * (1 - s), v * (1 - f * s), v * (1 - (1 - f) * s)
        r, g, b = [(v,t,p),(q,v,p),(p,v,t),(p,q,v),(t,p,v),(v,p,q)][i]
        return (round(r * 255), round(g * 255), round(b * 255))
    img = gradient((size, size), hsv(h, 0.55, 0.86), hsv((h + 28) % 360, 0.68, 0.62))
    d = ImageDraw.Draw(img)
    centred(d, (0, 0, size, size), initials(name), font(int(size * 0.40)), MIST)
    mask = Image.new("L", (size, size), 0)
    ImageDraw.Draw(mask).ellipse([0, 0, size - 1, size - 1], fill=255)
    out = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    out.paste(img, (0, 0), mask)
    out.save(path, optimize=True)

MERCHANTS = [
    ("MER-DEMO-0001", "Kacchi Bhai Dhanmondi",    "food"),
    ("MER-DEMO-0002", "Sultan's Dine Dhanmondi",  "food"),
    ("MER-DEMO-0003", "Star Kabab Gulshan",       "food"),
    ("MER-DEMO-0004", "Cheez Gulshan",            "food"),
    ("MER-DEMO-0005", "Shwapno Mirpur",           "grocery"),
    ("MER-DEMO-0006", "Agora Mirpur",             "grocery"),
    ("MER-DEMO-0007", "Lazz Pharma Uttara",       "pharmacy"),
    ("MER-DEMO-0008", "Unimart Uttara",           "grocery"),
    ("MER-DEMO-0009", "Nanna Biryani Motijheel",  "food"),
    ("MER-DEMO-0010", "Meena Bazar Mohammadpur",  "grocery"),
    ("MER-DEMO-0011", "Bhai Bhai Store Agrabad",  "grocery"),
    ("MER-DEMO-0012", "Khulshi Mart",             "grocery"),
    ("MER-DEMO-0013", "Panshi Restaurant",        "food"),
    ("MER-DEMO-0014", "Khulna Grocers",           "grocery"),
]

CATEGORIES = [
    ("food",     "Food",     "◆"),
    ("grocery",  "Grocery",  "▲"),
    ("pharmacy", "Pharmacy", "✚"),
    ("parcel",   "Parcel",   "■"),
]

AVATARS = [
    ("USR-DEMO-0001", "Ayesha Rahman"),
    ("USR-DEMO-0002", "Tanvir Hasan"),
    ("USR-DEMO-0003", "Nusrat Jahan"),
    ("RDR-DEMO-0001", "Rifat Islam"),
    ("RDR-DEMO-0002", "Sabbir Ahmed"),
]

manifest = {"merchants": {}, "categories": {}, "avatars": {}}

for mid, name, cat in MERCHANTS:
    lp = f"{ROOT}/merchants/{mid}-logo.png"
    cp = f"{ROOT}/merchants/{mid}-cover.png"
    logo(name, cat, lp)
    cover(name, cat, cp)
    manifest["merchants"][mid] = {
        "name": name, "category": cat,
        "logo": f"merchants/{mid}-logo.png",
        "cover": f"merchants/{mid}-cover.png",
    }

for slug, label, glyph in CATEGORIES:
    p = f"{ROOT}/categories/{slug}.png"
    category_tile(slug, label, glyph, p)
    manifest["categories"][slug] = {"label": label, "tile": f"categories/{slug}.png"}

for uid, name in AVATARS:
    p = f"{ROOT}/avatars/{uid}.png"
    avatar(name, p)
    manifest["avatars"][uid] = {"name": name, "image": f"avatars/{uid}.png"}

with open(f"{ROOT}/manifest.json", "w") as f:
    json.dump(manifest, f, indent=2, sort_keys=True)
    f.write("\n")

print("merchants", len(MERCHANTS), "categories", len(CATEGORIES), "avatars", len(AVATARS))
