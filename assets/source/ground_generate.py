#!/usr/bin/env python3
"""Original He Wan village ground paintings. No external art assets.

Run with CPython 3, numpy, scipy and Pillow from any working directory.
Outputs 2048² RGB PNGs and a tiled inspection sheet. Deterministic seed 41873.
A 2047-pixel torus is painted, then its initial row/column is repeated exactly.
Normal maps are tangent-space OpenGL (+Y/green up), in linear data encoding.
The derived heights are physical metres; the recommended tile size is 4 m.
"""
from pathlib import Path
import argparse, json, math, hashlib
import numpy as np
from PIL import Image, ImageDraw, ImageFont
from scipy.ndimage import gaussian_filter, distance_transform_edt, zoom
from scipy.spatial import cKDTree

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / 'client/assets/art/textures'
SOURCE = ROOT / 'assets/source'
N = 2047
SEED = 41873
TILE_METRES = 4.0
RNG = np.random.default_rng(SEED)
Y, X = np.mgrid[:N, :N].astype(np.float32)


def smoothstep(a,b,x):
    t=np.clip((x-a)/(b-a),0,1)
    return t*t*(3-2*t)


def noise(scale, amplitude=1.0, ratio=1.0):
    # Gaussian filtered white noise is periodic along both axes.
    a=RNG.normal(0,1,(N,N)).astype(np.float32)
    a=gaussian_filter(a, sigma=(scale,scale*ratio), mode='wrap')
    a/=max(float(a.std()),1e-8)
    return np.clip(a,-2.8,2.8)*amplitude


def edge_repeat(a):
    return np.concatenate([np.concatenate([a,a[:,:1]],axis=1),
                           np.concatenate([a[:1],a[:1,:1]],axis=1)],axis=0)


def periodic_draw(draw, points, fill, line=False, width=1):
    # Repaint crossing marks on every relevant torus copy.
    x0=min(p[0] for p in points); x1=max(p[0] for p in points)
    y0=min(p[1] for p in points); y1=max(p[1] for p in points)
    xs=[0]+([N] if x0<0 else [])+([-N] if x1>=N else [])
    ys=[0]+([N] if y0<0 else [])+([-N] if y1>=N else [])
    for dx in xs:
        for dy in ys:
            p=[(round(x+dx),round(y+dy)) for x,y in points]
            if line: draw.line(p,fill=fill,width=width)
            else: draw.polygon(p,fill=fill)


def clamp8(a): return np.clip(np.rint(a),0,255).astype(np.uint8)


def output(name, rgb, height):
    rgb=clamp8(rgb)
    # Image y is down, UV V is up: invert the y derivative relative to x.
    dhdx=(np.roll(height,-1,axis=1)-np.roll(height,1,axis=1))/(2*TILE_METRES/N)
    dhdy=(np.roll(height,-1,axis=0)-np.roll(height,1,axis=0))/(2*TILE_METRES/N)
    normal=np.stack([-dhdx,dhdy,np.ones_like(height)],axis=2)
    normal/=np.linalg.norm(normal,axis=2,keepdims=True)
    normal=clamp8((normal+1)*127.5)
    for suffix,pix in [('',rgb),('_normal',normal)]:
        Image.fromarray(edge_repeat(pix)).save(OUT/f'{name}{suffix}.png',compress_level=9)
    print(f'Painted {name}',flush=True)


def stone():
    # Thirty broad, lightly staggered fieldstones, not micro-cobbles.
    sites=[]
    for row in range(5):
        for col in range(6):
            sx=(col+0.48+(0.27 if row%2 else 0)+RNG.uniform(-.16,.16))*N/6
            sy=(row+0.5+RNG.uniform(-.17,.17))*N/5
            sites.append((sx%N,sy%N))
    sites=np.array(sites,np.float32)
    warp_x=noise(38,6.0); warp_y=noise(42,5.0)
    tree=cKDTree(sites,boxsize=N)
    labels=np.empty((N,N),dtype=np.int32)
    for start in range(0,N,128):
        end=min(N,start+128)
        pts=np.stack([(X[start:end]+warp_x[start:end])%N,
                      (Y[start:end]+warp_y[start:end])%N],axis=-1)
        labels[start:end]=tree.query(pts.reshape(-1,2),workers=1)[1].reshape(end-start,N)
    boundary=np.zeros((N,N),bool)
    for axis in [0,1]:
        boundary|=labels!=np.roll(labels,1,axis=axis)
        boundary|=labels!=np.roll(labels,-1,axis=axis)
    pad=360
    dist=distance_transform_edt(np.pad(~boundary,pad,mode='wrap'))[pad:-pad,pad:-pad].astype(np.float32)
    mottles=noise(65); brush=noise(10,1,4.0); granule=noise(1.1)
    chips=noise(2.2)
    edge=dist-1.6-chips*.8
    # Hand-shaped chips break the otherwise even mortar outline. Their RNG is
    # isolated so this refinement cannot change the other two paintings.
    chip_rng=np.random.default_rng(SEED+78)
    cut_image=Image.new('L',(N,N),0); cut_draw=ImageDraw.Draw(cut_image)
    by,bx=np.nonzero(boundary)
    for _ in range(470):
        bi=int(chip_rng.integers(len(bx))); x=float(bx[bi]); y=float(by[bi])
        site=sites[labels[int(y),int(x)]]
        dx=float((site[0]-x+N/2)%N-N/2); dy=float((site[1]-y+N/2)%N-N/2)
        norm=max(math.hypot(dx,dy),1); dx/=norm; dy/=norm
        width=chip_rng.uniform(3,10); length=chip_rng.uniform(5,18)
        pts=[(x-dy*width,y+dx*width),(x+dx*length-dy*width*.25,y+dy*length+dx*width*.25),
             (x+dx*length*.75+dy*width*.65,y+dy*length*.75-dx*width*.65),(x+dy*width,y-dx*width)]
        periodic_draw(cut_draw,pts,fill=int(chip_rng.integers(100,235)))
    cut=gaussian_filter(np.asarray(cut_image,np.float32)/255,.65,mode='wrap')
    bevel=smoothstep(1.5,13,edge-cut*9)
    colors=np.array([160,156,138],np.float32)+RNG.normal(0,[1.7,1.4,1.2],(len(sites),3))
    colors+=RNG.uniform(-9,9,(len(sites),1))
    faces=colors[labels]+mottles[:,:,None]*3.6+brush[:,:,None]*2.6+granule[:,:,None]*.65
    # Warm, muted edge catching and cool interior swaths read as brushed pigment.
    rim=np.exp(-((edge-14)/6)**2)*4.0
    faces+=rim[:,:,None]*np.array([1.0,.94,.75])
    # Brushed mineral bloom and a few chipped facets, rather than granular noise.
    mineral=smoothstep(.45,1.7,brush*.7+mottles*.35)
    faces+=mineral[:,:,None]*np.array([3.7,3.2,1.7])
    faces+=cut[:,:,None]*np.array([6.5,6.0,4.4])
    patina=smoothstep(.5,1.8,mottles)*smoothstep(-.5,1.5,-brush)
    faces-=patina[:,:,None]*np.array([3.5,2.2,1.1])
    grout=np.array([101,103,83],np.float32)+mottles[:,:,None]*2+granule[:,:,None]*1.1
    rgb=grout*(1-bevel[:,:,None])+faces*bevel[:,:,None]
    mossfield=noise(43)
    moss=smoothstep(.3,1.5,mossfield)*np.exp(-dist/10)*.72
    rgb=rgb*(1-moss[:,:,None])+np.array([95,108,65])*moss[:,:,None]
    # Thin hairline fractures are short and interrupted, limited to a few slabs.
    cracks=Image.new('L',(N,N),0); cd=ImageDraw.Draw(cracks)
    by,bx=np.nonzero(boundary)
    for _ in range(35):
        idx=int(RNG.integers(len(bx))); x=float(bx[idx]); y=float(by[idx])
        theta=RNG.uniform(0,2*np.pi); length=RNG.uniform(24,70)
        pts=[(x,y)]
        for j in range(1,5):
            t=j/4; pts.append((x+math.cos(theta)*length*t+RNG.normal(0,3),y+math.sin(theta)*length*t+RNG.normal(0,3)))
        periodic_draw(cd,pts,fill=int(RNG.integers(90,160)),line=True,width=int(RNG.integers(1,3)))
    crack=np.asarray(cracks,np.float32)/255
    crack=gaussian_filter(crack,.8,mode='wrap')*bevel
    rgb-=crack[:,:,None]*24
    height=.021*bevel+.0011*mottles*bevel+.00032*brush*bevel+.00009*granule*bevel-.002*crack
    output('ground_stone_paving',rgb,height)


def dirt():
    clouds=noise(105); patches=noise(30); brush=noise(5,1,.4); fine=noise(.9)
    rgb=np.zeros((N,N,3),np.float32)+[143,126,96]
    rgb+=clouds[:,:,None]*np.array([4.8,4.0,3.0])+patches[:,:,None]*2+brush[:,:,None]*.8+fine[:,:,None]*.45
    height=clouds*.0015+patches*.0007+brush*.00022+fine*.00005
    # Two shallow, gently curving wheel-worn tracks, periodic in travel direction.
    bend=np.sin(Y/N*2*np.pi)*26+np.sin(Y/N*6*np.pi+.7)*10
    rut_noise=noise(22)
    for center in [.27,.73]:
        dx=(X-center*N-bend+N/2)%N-N/2
        channel=np.exp(-(dx/(30+rut_noise*3))**2)
        shoulder=np.exp(-((np.abs(dx)-54)/24)**2)
        rgb-=channel[:,:,None]*np.array([6,5,3.5])
        rgb+=shoulder[:,:,None]*np.array([2.5,2.2,1.6])
        height-=channel*.004
    base=Image.fromarray(clamp8(rgb)); draw=ImageDraw.Draw(base)
    hm=Image.new('L',(N,N),0); hd=ImageDraw.Draw(hm)
    # Small hand-cut pebbles and sparse straw avoid photographic grit.
    for _ in range(430):
        x=RNG.uniform(0,N); y=RNG.uniform(0,N)
        rad=RNG.uniform(2.0,9.2); theta=RNG.uniform(0,2*np.pi)
        count=int(RNG.integers(5,8)); pts=[]
        for j in range(count):
            t=j/count*2*np.pi+theta; rr=rad*RNG.uniform(.72,1.15)
            pts.append((x+math.cos(t)*rr,y+math.sin(t)*rr*.7))
        shade=RNG.uniform(-9,14)
        col=tuple(clamp8(np.array([143,137,116])+shade))
        periodic_draw(draw,[(px+1,py+2) for px,py in pts],tuple(clamp8(np.array([109,100,76])+shade*.3)))
        periodic_draw(draw,pts,col)
        periodic_draw(hd,pts,int(RNG.uniform(80,180)))
        if rad>5:
            periodic_draw(draw,[(x-rad*.55,y-rad*.15),(x,y-rad*.5),(x+rad*.4,y-rad*.1)],tuple(clamp8(np.array(col)+[8,8,6])),line=True,width=1)
    # Low-contrast brush scratches vary length and don't become directional stripes.
    for _ in range(1450):
        x=RNG.uniform(0,N); y=RNG.uniform(0,N); length=RNG.uniform(4,23)
        col=tuple(clamp8(np.array([148,131,100])+RNG.uniform(-8,8)))
        periodic_draw(draw,[(x,y),(x+RNG.uniform(-3,3),y+length)],col,line=True,width=1)
    height+=gaussian_filter(np.asarray(hm,dtype=np.float32)/255,1.2,mode='wrap')*.005
    output('ground_dirt_road',np.asarray(base),height)


def grass():
    field=noise(155); patch=noise(40); broadbrush=noise(8,1,2.5); grain=noise(1.4)
    rgb=np.zeros((N,N,3),np.float32)+[84,105,58]
    rgb+=field[:,:,None]*np.array([4.5,4.2,2.1])+patch[:,:,None]*np.array([3.2,3.0,1.5])+broadbrush[:,:,None]*1.0+grain[:,:,None]*.35
    # Ground-level leaf cover is painted in nested tufts, with soft jade shadows.
    base=Image.fromarray(clamp8(rgb)); draw=ImageDraw.Draw(base)
    heights=Image.new('L',(N,N),65); hd=ImageDraw.Draw(heights)
    for _ in range(5900):
        x=RNG.uniform(0,N); y=RNG.uniform(0,N)
        local=field[int(y),int(x)]
        orientation=RNG.uniform(-.7,.7)
        size=RNG.uniform(.65,1.35)
        color_shift=RNG.uniform(-6,6)+local*2
        # Each blade has a gently curved spine and hand-tapered silhouette.
        for blade in range(int(RNG.integers(3,7))):
            angle=-math.pi/2+orientation+RNG.uniform(-1.0,1.0)
            length=RNG.uniform(11,28)*size; width=RNG.uniform(2.0,4.0)*size
            dx=math.cos(angle); dy=math.sin(angle); bend=RNG.uniform(-3.5,3.5)
            bx=x+RNG.uniform(-5,5); by=y+RNG.uniform(-4,4)
            tip=(bx+dx*length,by+dy*length)
            mid=(bx+dx*length*.5-dy*bend,by+dy*length*.5+dx*bend)
            pts=[(bx-dy*width*.5,by+dx*width*.5),
                 (mid[0]-dy*width*.6,mid[1]+dx*width*.6),tip,
                 (mid[0]+dy*width*.3,mid[1]-dx*width*.3),(bx+dy*width*.5,by-dx*width*.5)]
            lit=(blade%3)==0
            color=np.array([94,115,64] if lit else [74,97,52],np.float32)+color_shift
            if RNG.random()<.07: color=np.array([111,119,68],np.float32)+color_shift
            periodic_draw(draw,pts,tuple(clamp8(color)))
            periodic_draw(hd,pts,int(RNG.uniform(100,170)))
            if lit and length>17:
                periodic_draw(draw,[(bx,by),mid,tip],tuple(clamp8(color+[2,3,1])),line=True,width=1)
    height=field*.0011+patch*.0007+gaussian_filter(np.asarray(heights,np.float32)/255,.8,mode='wrap')*.0035
    output('ground_grass',np.asarray(base),height)


def validate():
    results={"generator":"assets/source/ground_generate.py","seed":SEED,
             "authoring":"Original deterministic procedural painting; no third-party textures or brushes",
             "reference":"assets/concept-art/starter-village-wide-v1.png; inspected visually",
             "tile_metres":TILE_METRES,"normal_convention":"OpenGL tangent space, +Y green up, linear/data",
             "periodic_domain":"2047 unique samples plus duplicate first row and column; zero opposite-edge pixel error",
             "files":[]}
    for p in sorted(OUT.glob('ground*.png')):
        a=np.array(Image.open(p)); raw=p.read_bytes()
        entry={"file":str(p.relative_to(ROOT)),"dimensions":list(a.shape[:2][::-1]),"mode":"RGB","bytes":len(raw),"sha256":hashlib.sha256(raw).hexdigest(),
               "edge_max_error_left_right":int(np.abs(a[:,0].astype(int)-a[:,-1].astype(int)).max()),
               "edge_max_error_top_bottom":int(np.abs(a[0].astype(int)-a[-1].astype(int)).max()),
               "channel_min":a.min(axis=(0,1)).tolist(),"channel_max":a.max(axis=(0,1)).tolist()}
        assert a.shape==(2048,2048,3)
        assert not entry['edge_max_error_left_right'] and not entry['edge_max_error_top_bottom']
        assert len(raw)<25*1024*1024
        if '_normal' in p.stem:
            vectors=a.astype(np.float32)/127.5-1
            lengths=np.linalg.norm(vectors,axis=2)
            entry['normal_length_min_max']=[float(lengths.min()),float(lengths.max())]
            entry['normal_positive_z']=bool((a[:,:,2]>=128).all())
            assert entry['normal_positive_z'] and lengths.min()>.99 and lengths.max()<1.01
        results['files'].append(entry)
    (ROOT/'docs/art/ground-validation.json').write_text(json.dumps(results,indent=2)+'\n')
    print('Verified 6 textures: 2048² RGB, exact edge matches, positive-Z unit normals, <25 MiB each',flush=True)


def preview():
    w=700; h=700; gap=24
    sheet=Image.new('RGB',(3*w+4*gap,2*h+140),(26,34,32)); d=ImageDraw.Draw(sheet)
    try: font=ImageFont.truetype('/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf',22)
    except OSError: font=ImageFont.load_default()
    names=['ground_stone_paving','ground_dirt_road','ground_grass']
    for j,(name,title) in enumerate(zip(names,['Warm river stone · 2 × 2 tiles','Wheel-worn loam · 2 × 2 tiles','Jade–olive meadow · 2 × 2 tiles'])):
        a=Image.open(OUT/f'{name}.png'); tile=a.resize((w//2,h//2),Image.Resampling.LANCZOS)
        comp=Image.new('RGB',(w,h))
        for i in range(2):
            for k in range(2): comp.paste(tile,(i*w//2,k*h//2))
        x=gap+j*(w+gap); sheet.paste(comp,(x,55)); d.text((x,20),title,font=font,fill=(219,221,199))
        normal=Image.open(OUT/f'{name}_normal.png').resize((w,h),Image.Resampling.LANCZOS)
        sheet.paste(normal,(x,h+95)); d.text((x,h+60),'OpenGL normal · +Y up · 4 m / tile',font=font,fill=(182,193,181))
    sheet.save(SOURCE/'ground-preview.png',compress_level=9)


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--validate-only',action='store_true')
    parser.add_argument('--stone-only',action='store_true',help='Refine only the stone pair; other paintings are untouched')
    args=parser.parse_args()
    OUT.mkdir(parents=True,exist_ok=True); SOURCE.mkdir(parents=True,exist_ok=True)
    if not args.validate_only:
        stone()
        if not args.stone_only: dirt(); grass()
    validate(); preview()
