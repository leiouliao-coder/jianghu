"""Build original Riverbend village architecture. Blender 4.3.2 background.
Usage: blender --background --python assets/source/buildings/build_buildings.py
Every asset is authored at metre scale, origin ground centre, south/front=-Y.
All surface art is original deterministic painted PNG; no external resources.
"""
import bpy, math, random, json, os
from pathlib import Path
from mathutils import Vector
ROOT=Path(__file__).resolve().parents[3]
SRC=ROOT/'assets/source/buildings'; TEX=SRC/'textures'; OUT=ROOT/'client/assets/art/village'; PRE=SRC/'renders'
OUT.mkdir(parents=True,exist_ok=True); PRE.mkdir(exist_ok=True)
random.seed(892)
MATS={}; B={}; MODEL=''

def material(name):
    if name in MATS:return MATS[name]
    m=bpy.data.materials.new('Painted_'+name);m.use_nodes=True;n=m.node_tree.nodes;n.clear();l=m.node_tree.links
    out=n.new('ShaderNodeOutputMaterial');p=n.new('ShaderNodeBsdfPrincipled');l.new(p.outputs['BSDF'],out.inputs['Surface'])
    for suffix,socket in [('basecolor','Base Color'),('normal',None),('orm',None)]:
        im=bpy.data.images.load(str(TEX/(name+'_'+suffix+'.png')),check_existing=True);im.pack()
        if suffix!='basecolor':im.colorspace_settings.name='Non-Color'
        t=n.new('ShaderNodeTexImage');t.image=im;t.label='Original painted '+suffix
        if suffix=='basecolor':l.new(t.outputs['Color'],p.inputs[socket])
        if suffix=='normal':
            no=n.new('ShaderNodeNormalMap');no.inputs['Strength'].default_value=.42;l.new(t.outputs['Color'],no.inputs['Color']);l.new(no.outputs[0],p.inputs['Normal'])
        if suffix=='orm':
            sep=n.new('ShaderNodeSeparateColor');l.new(t.outputs['Color'],sep.inputs[0]);l.new(sep.outputs[1],p.inputs['Roughness']);l.new(sep.outputs[2],p.inputs['Metallic'])
    if name=='ember':p.inputs['Emission Color'].default_value=(1,.19,.009,1);p.inputs['Emission Strength'].default_value=3.2
    m.diffuse_color={'wood':(.16,.09,.045,1),'plaster':(.7,.62,.45,1),'roof':(.14,.2,.22,1)}.get(name,(.3,.3,.3,1))
    MATS[name]=m;return m

def face(group,mat,vs,uv=None,smooth=False):
    k=(group,mat);d=B.setdefault(k,{'v':[],'f':[],'uv':[],'s':[]});i=len(d['v']);d['v']+=list(map(tuple,vs));d['f'].append(tuple(range(i,i+len(vs))))
    d['uv'].append(uv or ([(0,0),(1,0),(1,1),(0,1)][:len(vs)] if len(vs)<=4 else [(i/(len(vs)-1),0) for i in range(len(vs))]));d['s'].append(smooth)

def box(group,mat,c,d,rz=0):
    x,y,z=c;a,b,h=[v/2 for v in d];co=math.cos(rz);si=math.sin(rz)
    vs=[(x+co*xx-si*yy,y+si*xx+co*yy,z+zz) for xx,yy,zz in [(-a,-b,-h),(a,-b,-h),(a,b,-h),(-a,b,-h),(-a,-b,h),(a,-b,h),(a,b,h),(-a,b,h)]]
    for ids in [(0,3,2,1),(0,1,5,4),(1,2,6,5),(2,3,7,6),(3,0,4,7),(4,5,6,7)]:face(group,mat,[vs[i] for i in ids])

def beam(group,mat,a,b,w,depth=None):
    a=Vector(a);b=Vector(b);q=(b-a).to_track_quat('Z','Y');r=w/2;dr=(depth or w)/2;L=(b-a).length/2;c=(a+b)/2
    v=[c+q@Vector(p) for p in [(-r,-dr,-L),(r,-dr,-L),(r,dr,-L),(-r,dr,-L),(-r,-dr,L),(r,-dr,L),(r,dr,L),(-r,dr,L)]]
    for ids in [(0,3,2,1),(0,1,5,4),(1,2,6,5),(2,3,7,6),(3,0,4,7),(4,5,6,7)]:face(group,mat,[v[i] for i in ids])

def cylinder(group,mat,c,r,h,n=12,axis=(0,0,1),r2=None):
    q=Vector(axis).to_track_quat('Z','Y');c=Vector(c);r2=r if r2 is None else r2
    lo=[c+q@Vector((math.cos(i*2*math.pi/n)*r,math.sin(i*2*math.pi/n)*r,-h/2)) for i in range(n)]
    hi=[c+q@Vector((math.cos(i*2*math.pi/n)*r2,math.sin(i*2*math.pi/n)*r2,h/2)) for i in range(n)]
    for i in range(n):j=(i+1)%n;face(group,mat,[lo[i],lo[j],hi[j],hi[i]],[(i/n,0),(j/n,0),(j/n,1),(i/n,1)],True)
    for i in range(1,n-1):face(group,mat,[lo[0],lo[i+1],lo[i]],[(0,0),(1,0),(1,1)]);face(group,mat,[hi[0],hi[i],hi[i+1]],[(0,0),(1,0),(1,1)])

def tube(group,mat,points,r=.1,n=6):
    # A continuous tube avoids internal end caps at every segment.
    rings=[]
    for i,p in enumerate(points):
        tangent=Vector(points[min(i+1,len(points)-1)])-Vector(points[max(0,i-1)])
        q=tangent.to_track_quat('Z','Y');c=Vector(p)
        rings.append([c+q@Vector((r*math.cos(j*2*math.pi/n),r*math.sin(j*2*math.pi/n),0)) for j in range(n)])
    for i in range(len(rings)-1):
        for j in range(n):
            k=(j+1)%n;face(group,mat,[rings[i][j],rings[i][k],rings[i+1][k],rings[i+1][j]],[(j/n,0),(k/n,0),(k/n,1),(j/n,1)],True)
    for i in range(1,n-1):
        face(group,mat,[rings[0][0],rings[0][i+1],rings[0][i]],[(0,0),(1,0),(1,1)])
        face(group,mat,[rings[-1][0],rings[-1][i],rings[-1][i+1]],[(0,0),(1,0),(1,1)])

def hip_roof(name,a,b,eave,ridge,r=.3,cy=0,cols=30,rows=7):
    # Four curved roof panels. Each individual roll tile has a raised rounded crown.
    ridge_x=a*.43
    def pos(side,u,t):
        width=(1-t)*(a if side<2 else b)+t*(ridge_x if side<2 else 0)
        z=eave+(ridge-eave)*t**1.68+.29*abs(u)**7*(1-t)**3
        if side<2:return Vector((u*width,cy+(-1 if side==0 else 1)*(1-t)*b,z))
        return Vector(((-1 if side==2 else 1)*((1-t)*a+t*ridge_x),cy+u*width,z))
    for s in range(4):
        ncol=cols if s<2 else max(16,int(cols*b/a));nrow=rows if s<2 else max(4,rows-1)
        # continuous dark underroof fills any tiny tile gaps and gives eave thickness
        for j in range(4):
            for i in range(4):
                u0=-1+2*i/4;u1=-1+2*(i+1)/4;t0=j/4;t1=(j+1)/4
                p=[pos(s,u0,t0),pos(s,u1,t0),pos(s,u1,t1),pos(s,u0,t1)]
                face(name+'_underroof','wood',[v-Vector((0,0,.07)) for v in p])
        # Barrel tile columns stay parallel; hip boundaries clip rows instead of fanning every tile into a point.
        base_width=a if s<2 else b
        tile_width=2*base_width/ncol
        def physical(q,t):
            width=(1-t)*base_width+t*(ridge_x if s<2 else 0)
            return pos(s,max(-width,min(width,q))/width if width>1e-7 else 0,t)
        for j in range(nrow):
            t0=j/nrow;t1=(j+1)/nrow
            width0=(1-t0)*base_width+t0*(ridge_x if s<2 else 0)
            width1=(1-t1)*base_width+t1*(ridge_x if s<2 else 0)
            for i in range(ncol):
                q0=-base_width+i*tile_width;q1=q0+tile_width
                if q0>=max(width0,width1) or q1<=-max(width0,width1):continue
                tile_mat=random.choices(['roof','roof_light','roof_dark','roof_warm'],[5,2,2,1])[0]
                front=[]
                for k in range(3):
                    s0=k/3;s1=(k+1)/3;pts=[]
                    for ss,t in [(s0,t0),(s1,t0),(s1,t1),(s0,t1)]:
                        pp=physical(q0+(q1-q0)*ss,t)
                        pp.z+=.035+.14*math.sin(ss*math.pi)+(.035 if t==t0 else 0)
                        pts.append(pp)
                    face(name+'_rolled_tiles',tile_mat,pts,[(s0,0),(s1,0),(s1,1),(s0,1)],True)
                    if k==0:front.append(pts[0])
                    front.append(pts[1])
                # Physical lower cut face gives an overlap shadow at every tile row.
                lip=front+[front[-1]-Vector((0,0,.065)),front[0]-Vector((0,0,.065))]
                face(name+'_tile_overlap_lips',tile_mat,lip,[(0,1),(1/3,1),(2/3,1),(1,1),(1,0),(0,0)])
        # Shape-defining raised verge and eave trim, six-sided deliberate silhouette
        tube(name+'_eave_trim','roof',[pos(s,-1+2*i/14,0)+Vector((0,0,.015)) for i in range(15)],.09,6)
    for sg in [-1,1]:
        for sy in [-1,1]:
            # Diagonal hip cap follows matching curve from corner to ridge end.
            pts=[]
            for i in range(9):
                t=i/8;pts.append((sg*((1-t)*a+t*ridge_x),cy+sy*b*(1-t),eave+(ridge-eave)*t**1.68+.29*(1-t)**3+.11))
            tube(name+'_hip_caps','roof',pts,.12,6)
    # Segmented substantial roof ridge, with restrained upturned end pieces.
    for i in range(18):
        x=-ridge_x+2*ridge_x*(i+.5)/18;box(name+'_ridge','roof',(x,cy,ridge+.12),(2*ridge_x/18-.015,.3,.22))
    for s in [-1,1]:
        tube(name+'_ridge_finials','roof',[(s*ridge_x,cy,ridge+.12),(s*(ridge_x+.2),cy,ridge+.28),(s*(ridge_x+.30),cy,ridge+.45)],.13,6)

# All facade details live inside the 14m structural envelope.
def facade_box(group,mat,side,u,z,w,h,depth=.12,rad=6.92):
    if side==0:box(group,mat,(u,-rad,z),(w,depth,h))
    if side==1:box(group,mat,(u,rad,z),(w,depth,h))
    if side==2:box(group,mat,(-rad,u,z),(depth,w,h))
    if side==3:box(group,mat,(rad,u,z),(depth,w,h))

def window(side,u,z,w=2.05,h=1.6,rad=6.91):
    facade_box('Window_recess','wood',side,u,z,w+.2,h+.2,.08,rad-.02)
    facade_box('Warm_paper_panels','paper',side,u,z,w,h,.06,rad+.005)
    for du in [-w/2,w/2]:facade_box('Window_frames','wood',side,u+du,z,.12,h+.18,.12,rad+.04)
    for dz in [-h/2,h/2]:facade_box('Window_frames','wood',side,u,z+dz,w,.12,.12,rad+.044)
    for i in range(1,6):facade_box('Window_lattice','wood',side,u-w/2+i*w/6,z,.045,h,.06,rad+.04)
    for j in range(1,5):facade_box('Window_lattice','wood',side,u,z-h/2+j*h/5,w,.047,.06,rad+.055)
    facade_box('Window_sill','wood',side,u,z-h/2-.12,w+.35,.16,.14,rad+.015)

def tiled_floor(z=.12):
    box('Foundation','stone',(0,0,z),(14,14,z*2))
    # Large worn flagstones bounded precisely within footprint.
    for i in range(14):
        for j in range(14):box('Flagstone_paving','stone',(-6.5+i,-6.5+j,2*z+.035),(.98,.98,.07))

def lantern(x,y,z):
    cylinder('Lantern_body','cloth',(x,y,z),.24,.52,10,r2=.24)
    for dz in [-.29,.29]:cylinder('Lantern_caps','brass',(x,y,z+dz),.2,.06,10)
    cylinder('Lantern_tassel','cloth',(x,y,z-.43),.045,.22,6)
    beam('Lantern_hook','iron',(x,y,z+.31),(x,y,z+.7),.035)
    for i in range(6):
        a=math.pi*2*i/6;beam('Lantern_ribs','brass',(x+.245*math.cos(a),y+.245*math.sin(a),z-.23),(x+.245*math.cos(a),y+.245*math.sin(a),z+.23),.012)

def hall():
    random.seed(892)
    # Continuous exterior envelope with true recessed window/door openings.
    # Inset plaster panels sit behind proud solid corner timbers; ground base is exactly 14m.
    for side in range(4):
        holes=[(u-1.22,u+1.22,4.1,5.9) for u in [-4.95,-1.7,1.7,4.95]]
        holes += [(u-1.22,u+1.22,1.13,2.95) for u in ([-4.95,4.95] if side==0 else [-4.95,-1.7,1.7,4.95])]
        if side==0:holes.append((-1.77,1.77,.35,3.36))
        xs=sorted(set([-7,7]+[v for h in holes for v in h[:2]]));zs=sorted(set([.3,6.34]+[v for h in holes for v in h[2:]]))
        for x0,x1 in zip(xs,xs[1:]):
            for z0,z1 in zip(zs,zs[1:]):
                x=(x0+x1)/2;z=(z0+z1)/2
                if any(h[0]<x<h[1] and h[2]<z<h[3] for h in holes):continue
                # Only visible front/back sheets; no redundant faces inside a continuous wall.
                for rad in [6.94,6.60]:
                    v=[(x0,-rad,z0),(x1,-rad,z0),(x1,-rad,z1),(x0,-rad,z1)]
                    if side==1:v=[(xx,-yy,zz) for xx,yy,zz in v]
                    if side==2:v=[(yy,xx,zz) for xx,yy,zz in v]
                    if side==3:v=[(-yy,xx,zz) for xx,yy,zz in v]
                    face('Solid_wall_'+str(side),'plaster',v)

    # Exact solid corner envelope gives +/-7m, with no projecting decoration.
    for x in [-6.8,6.8]:
        for y in [-6.8,6.8]:
            box('Solid_corner_posts','wood',(x,y,3.57),(.4,.4,5.54))
            box('Solid_corner_stone_feet','stone',(x,y,.58),(.4,.4,.44))
    box('Hall_foundation','stone',(0,0,.18),(14,14,.36))
    # Main posts inset into plaster, relief finishes never exceed 14m.
    for side in range(4):
        for u in [-6.78,-3.4,0,3.4,6.78]:
            if abs(u)<6:
                if side==0 and abs(u)<.01:facade_box('Timber_posts','wood',side,u,5.22,.4,2.24,.18,6.89)
                else:facade_box('Timber_posts','wood',side,u,3.55,.4,5.6,.18,6.89)
                if not(side==0 and abs(u)<.01):facade_box('Post_stone_feet','stone',side,u,.55,.44,.45,.19,6.9)
            for z in [3.15,6.08]:
                facade_box('Bracket_blocks','wood',side,u,z,.7,.19,.18,6.906)
                facade_box('Bracket_blocks','wood',side,u,z+.18,.98,.14,.18,6.906)
        for z in [.88,3.05,3.42,4.0,6.16]:facade_box('Horizontal_timber_bands','wood',side,0,z,13.6,.18,.12,6.93)
        for u in [-4.95,-1.7,1.7,4.95]:
            window(side,u,5.0,2.25,1.55,6.89)
            if side!=0 or abs(u)>3:window(side,u,2.04,2.25,1.55,6.89)
    # Closed double doorway prevents an invisible-wall entrance mismatch.
    facade_box('Door_recess','wood',0,0,1.6,3.5,2.5,.1,6.8)
    for i in range(12):facade_box('Door_planks','wood',0,-1.5+(i+.5)*3/12,1.63,3/12-.02,2.45,.1,6.91)
    for u in [-1.65,1.65]:facade_box('Door_jamb','wood',0,u,1.7,.18,2.75,.1,6.947)
    for x in [-.2,.2]:
        cylinder('Door_boss','brass',(x,-6.975,1.55),.09,.025,10,(0,-1,0))
        # Stylized raised knockers without unsupported curve export.
        tube('Door_knockers','iron',[(x+.075*math.cos(i*2*math.pi/10),-6.98,1.45+.09*math.sin(i*2*math.pi/10)) for i in range(11)],.018,5)
    # Plaque remains intentionally unlettered for localization.
    facade_box('Blank_plaque','wood',0,0,3.02,3.45,.62,.14,6.922)
    for z in [2.73,3.31]:facade_box('Plaque_brass_border','brass',0,0,z,3.4,.035,.035,6.981)
    for x in [-1.68,1.68]:facade_box('Plaque_brass_border','brass',0,x,3.02,.035,.58,.035,6.981)
    for x in [-5.9,5.9]:lantern(x,-6.7,2.75)
    # Lower eaves are a narrow ring, not a full intersecting hidden roof.
    for side in range(4):
        def ringpos(u,t):
            a=7.78-.78*t;z=3.48+.53*t+.17*abs(u)**8*(1-t)
            if side<2:return Vector((u*a,(-1 if side==0 else 1)*a,z))
            return Vector(((-1 if side==2 else 1)*a,u*a,z))
        for i in range(38):
            for j in range(2):
                for k in range(2):
                    u0=-1+2*(i+k/2)/38;u1=-1+2*(i+(k+1)/2)/38;t0=j/2;t1=(j+1)/2
                    vs=[ringpos(u0,t0),ringpos(u1,t0),ringpos(u1,t1),ringpos(u0,t1)]
                    for vi,v in enumerate(vs):v.z+=.12*math.sin((k+(1 if vi in [1,2] else 0))*math.pi/2)
                    face('Lower_eave_tiles','roof',vs,[(k/2,0),((k+1)/2,0),((k+1)/2,1),(k/2,1)],True)
        tube('Lower_eave_edge','roof',[ringpos(-1+2*i/18,0) for i in range(19)],.08,6)
    hip_roof('Hall_main_roof',7.78,7.78,6.28,8.55,cols=34,rows=8)

def forge():
    random.seed(892)
    tiled_floor()
    # Rear workshop occupies northern half; south canopy is open and populated.
    box('Workshop_rear_wall','plaster',(0,6.5,1.99),(13.3,.3,3.38))
    for x in [-6.5,6.5]:box('Workshop_side_wall','plaster',(x,3.7,1.99),(.3,5.3,3.38))
    box('Workshop_front_wall','plaster',(0,.9,1.99),(13.3,.3,3.38))
    # Original timber-framed side elevations; all ornament stays inside the 14m platform.
    for side in [2,3]:
        for u in [2.15,4.85]:window(side,u,2.05,1.85,1.5,6.68)
        for u in [.9,3.5,6.25]:facade_box('Workshop_side_timbers','wood',side,u,2.0,.22,3.35,.18,6.65)
        for z in [.75,3.35]:facade_box('Workshop_side_timbers','wood',side,3.6,z,5.65,.18,.18,6.66)
        facade_box('Workshop_stone_course','stone',side,3.6,.48,5.65,.35,.16,6.64)
    # Closed rear workshop door and front-facing windows, set before broad canopy.
    for i in range(9):box('Workshop_door','wood',(.8+(i+.5)*.25,.715,1.56),(.235,.1,2.5))
    for x in [.75,3.1]:box('Workshop_door_jamb','wood',(x,.68,1.56),(.16,.16,2.65))
    for x in [-6.25,-2.1,2.1,6.25]:
        for y in [-5.1,.6,6.25]:
            if y==.6 and abs(x)<6:continue
            box('Canopy_stone_feet','stone',(x,y,.48),(.58,.58,.34))
            box('Canopy_timber_posts','wood',(x,y,2.0),(.36,.36,3.25))
            for z in [.78,3.25]:box('Post_iron_straps','iron',(x,y,z),(.39,.39,.13))
            for sx in [-1,1]:beam('Canopy_corner_braces','wood',(x,y,2.75),(x+sx*.6,y,3.43),.19)
            for sy in [-1,1]:beam('Canopy_corner_braces','wood',(x,y,2.75),(x,y+sy*.65,3.43),.19)
    for y in [-5.1,.6,6.25]:box('Canopy_crossbeams','wood',(0,y,3.43),(12.9,.32,.35))
    for x in [-6.25,6.25]:box('Canopy_longbeams','wood',(x,.57,3.43),(.32,11.7,.35))
    # Roof leaves a functional southern apron, displaying the anvil at game pitch.
    hip_roof('Forge_main_roof',6.72,6.3,3.65,5.5,cy=.6,cols=28,rows=6)
    # Mortared stone forge hearth and hood, chimney not taller than 6.0m.
    hx,hy=-3.65,-.45
    box('Hearth_foot','stone',(hx,hy,.62),(2.7,1.65,.66))
    box('Hearth_back','stone',(hx,hy+.65,1.77),(2.7,.28,2.3))
    for x in [hx-1.18,hx+1.18]:box('Hearth_cheeks','stone',(x,hy,1.45),(.35,1.45,1.6))
    box('Hearth_hood','stone',(hx,hy,2.75),(2.8,1.65,.45))
    box('Chimney','stone',(hx,hy+.35,4.25),(.85,.85,2.8))
    box('Chimney_cap','iron',(hx,hy+.35,5.72),(1.02,1.02,.18))
    box('Chimney_dark_mouth','coal',(hx,hy+.35,5.82),(.75,.75,.015))
    box('Ember_bed','ember',(hx,hy-.2,1.02),(1.9,1.1,.14))
    for i in range(45):
        x=hx+random.uniform(-.85,.85);y=hy+random.uniform(-.65,.1);z=1.1+random.uniform(0,.1)
        cylinder('Glowing_coals','ember' if i%5==0 else 'coal',(x,y,z),random.uniform(.11,.17),random.uniform(.06,.17),6,r2=.04)
    for i in range(4):cylinder('Forge_flames','ember',(hx-.6+i*.4,hy-.2,1.32),.12,.46,5,r2=.015)
    # Stone coal bin with entirely visible stock.
    bx,by=-5.2,-3.1
    box('Coal_bin_base','stone',(bx,by,.4),(1.55,1.4,.2))
    for x in [bx-.68,bx+.68]:box('Coal_bin_wall','stone',(x,by,.8),(.18,1.04,.6))
    for y in [by-.61,by+.61]:box('Coal_bin_wall','stone',(bx,y,.8),(1.55,.18,.6))
    for i in range(18):cylinder('Coal_stock','coal',(bx+random.uniform(-.5,.5),by+random.uniform(-.45,.45),.83+random.random()*.15),.15,.16,5,r2=.1)
    # Horned forged anvil on pegged timber stump, sculpted profile rather than a box.
    ax,ay=-.7,-5.65
    cylinder('Anvil_stump','wood',(ax,ay,.75),.67,.85,12,r2=.63)
    for z in [.42,.99]:cylinder('Stump_iron_hoops','iron',(ax,ay,z),.68,.075,12)
    box('Anvil_feet','iron',(ax,ay,1.22),(1.15,.72,.18));box('Anvil_waist','iron',(ax,ay,1.42),(.55,.48,.42));box('Anvil_face','iron',(ax,ay,1.71),(1.3,.63,.21))
    # tapering pyramidal horn, directed east, with hammered faceted silhouette
    horn=[(ax+.62,ay-.29,1.6),(ax+.62,ay+.29,1.6),(ax+.62,ay+.29,1.8),(ax+.62,ay-.29,1.8),(ax+1.36,ay,1.73)]
    for f in [(0,1,4),(1,2,4),(2,3,4),(3,0,4)]:face('Anvil_horn','iron',[horn[i] for i in f],[(0,0),(1,0),(.5,1)])
    box('Anvil_heel','iron',(ax-.85,ay,1.71),(.42,.52,.21))
    # Workbench on east side, tools lying on top, spare blades and ingots below.
    tx,ty=4.4,-2.4
    for x in [tx-1.1,tx+1.1]:
        for y in [ty-.45,ty+.45]:box('Workbench_legs','wood',(x,y,.95),(.18,.18,1.25))
    for j in range(4):box('Workbench_planks','wood',(tx,ty-.47+j*.31,1.58),(2.6,.3,.14))
    box('Workbench_shelf','wood',(tx,ty,.63),(2.6,1.2,.12))
    for i in range(7):box('Metal_ingots','iron',(tx-1+i*.31,ty,.77),(.27,.6,.15))
    for i in range(3):
        beam('Workbench_hammer_handles','wood',(tx-.8+i*.7,ty-.4,1.73),(tx-.65+i*.7,ty+.35,1.73),.06)
        box('Workbench_hammer_heads','iron',(tx-.65+i*.7,ty+.35,1.76),(.3,.16,.16))
    for x in [-1.6,-1.1,-.6]:
        beam('Hanging_tool_handles','wood',(x,.55,1.2),(x,.55,2.6),.055)
        box('Hanging_tool_heads','iron',(x,.55,2.5),(.22,.12,.25))
    # Stock rack within right edge, plus piled billets at the back.
    for x in [4.5,5.8]:box('Stock_rack','wood',(x,-.3,.9),(.15,.15,1.2))
    for z in [.65,1.3]:box('Stock_rack','wood',(5.15,-.3,z),(1.45,.18,.14))
    for i in range(10):beam('Steel_rod_stock','iron',(4.55+i*.125,-.45,.35),(4.65+i*.125,-.26,2.0+random.random()*.4),.055)
    for i in range(8):cylinder('Fuel_billets','wood',(3.65+(i%4)*.32,4.9, .5+(i//4)*.27),.15,1.45,7,(0,1,0))
    # Small bucket beside anvil.
    cylinder('Quench_bucket','wood',(1.3,-5.6,.64),.42,.65,12,r2=.48)
    for z,r in [(.4,.44),(.89,.48)]:cylinder('Bucket_hoops','iron',(1.3,-5.6,z),r,.06,12)
    cylinder('Dark_water','iron',(1.3,-5.6,.97),.4,.015,12)
    # Blank red banner stays well within the platform.
    box('Blank_red_banner','cloth',(-6.22,-4.82,2.52),(.65,.045,1.25))
    beam('Banner_rod','wood',(-6.62,-4.82,3.2),(-5.82,-4.82,3.2),.06)
    # Rear/side windows oriented to workshop walls.
    for x in [-4.2,4.2]:
        box('Rear_lattice_recess','wood',(x,6.66,2.12),(2.1,.08,1.5))
        for j in range(6):box('Rear_lattice','wood',(x-.9+j*.36,6.72,2.12),(.045,.04,1.4))
        for j in range(4):box('Rear_lattice','wood',(x,6.72,1.5+j*.4),(2.0,.04,.045))


def clear_scene():
    bpy.ops.object.select_all(action='SELECT');bpy.ops.object.delete(use_global=False)
    for d in bpy.data.meshes:
        if d.users==0:bpy.data.meshes.remove(d)

def make_meshes(name):
    root=bpy.data.objects.new(name,None);bpy.context.collection.objects.link(root);root['asset_type']='original_wuxia_building';root['footprint_metres']=[14.,14.];root['front']='-Y';root['ground_origin']=True
    result=[]
    for (group,mat),d in B.items():
        me=bpy.data.meshes.new(group);me.from_pydata(d['v'],[],d['f']);me.materials.append(material(mat));me.update()
        uv=me.uv_layers.new(name='PaintedUV')
        for poly,uvs,sm in zip(me.polygons,d['uv'],d['s']):
            poly.use_smooth=sm
            for li,uvco in zip(poly.loop_indices,uvs):uv.data[li].uv=uvco
        ob=bpy.data.objects.new(group,me);bpy.context.collection.objects.link(ob);ob.parent=root;result.append(ob)
    return root,result

def aim(ob,pt):ob.rotation_euler=(Vector(pt)-ob.location).to_track_quat('-Z','Y').to_euler()

def render_setup(name):
    scene=bpy.context.scene;scene.render.engine='CYCLES';scene.cycles.samples=32;scene.cycles.use_denoising=False
    scene.render.resolution_x=1400;scene.render.resolution_y=1100;scene.render.resolution_percentage=100
    scene.world.color=(.65,.65,.65);scene.view_settings.view_transform='AgX'
    w=scene.world;w.use_nodes=True;w.node_tree.nodes['Background'].inputs[0].default_value=(.68,.73,.77,1);w.node_tree.nodes['Background'].inputs[1].default_value=.85
    bpy.ops.object.light_add(type='AREA',location=(-8,-10,21));bpy.context.object.name='Preview_Key';bpy.context.object.data.energy=2300;bpy.context.object.data.shape='DISK';bpy.context.object.data.size=12;aim(bpy.context.object,(0,0,2))
    bpy.ops.object.light_add(type='AREA',location=(10,5,14));bpy.context.object.name='Preview_Fill';bpy.context.object.data.energy=1300;bpy.context.object.data.size=10;aim(bpy.context.object,(0,0,3))
    bpy.ops.object.camera_add(location=(20,-27,53));cam=bpy.context.object;cam.name='Preview_Camera_55deg';cam.data.type='ORTHO';cam.data.ortho_scale=29;aim(cam,(0,0,5));scene.camera=cam
    # neutral backdrop is preview-only and explicitly omitted from GLB
    bpy.ops.mesh.primitive_plane_add(size=2000,location=(0,0,-.015));plane=bpy.context.object;plane.name='Preview_Backdrop';m=bpy.data.materials.new('Preview_Backdrop');m.diffuse_color=(.42,.43,.38,1);plane.data.materials.append(m)
    scene.render.filepath=str(PRE/(name+'_oblique.png'));bpy.ops.render.render(write_still=True)
    cam.location=(0,-32,4.5);aim(cam,(0,0,4.5 if name=='building_hall' else 3));cam.data.ortho_scale=20;scene.render.filepath=str(PRE/(name+'_front.png'));bpy.ops.render.render(write_still=True)
    cam.location=(32,0,4.5);aim(cam,(0,0,4.5 if name=='building_hall' else 3));scene.render.filepath=str(PRE/(name+'_side.png'));bpy.ops.render.render(write_still=True)
    for ob in list(bpy.data.objects):
        if ob.name.startswith('Preview_'):bpy.data.objects.remove(ob,do_unlink=True)

report_path=ROOT/'docs/art/buildings-validation.json'
report=json.loads(report_path.read_text()) if report_path.exists() else {}
for name,fn in [('building_hall',hall),('building_forge',forge)]:
    if os.environ.get('ASSET_FILTER') and name!=os.environ['ASSET_FILTER']:continue
    clear_scene();B={};fn();root,obs=make_meshes(name)
    # Apply transforms and triangulate explicitly, with no remaining modifiers.
    for ob in obs:
        bpy.context.view_layer.objects.active=ob;ob.select_set(True)
        tri=ob.modifiers.new('Applied_triangulation','TRIANGULATE');bpy.ops.object.modifier_apply(modifier=tri.name);ob.select_set(False)
    verts=[ob.matrix_world@v.co for ob in obs for v in ob.data.vertices]
    low=[min(v[i] for v in verts) for i in range(3)];high=[max(v[i] for v in verts) for i in range(3)]
    tris=sum(len(ob.data.polygons) for ob in obs)
    report[name]={'triangles':tris,'bounds_blender_min':low,'bounds_blender_max':high,'dimensions':[high[i]-low[i] for i in range(3)],'footprint':[14,14],'front':'-Y','origin':[0,0,0],'materials':sorted(set(mat for _,mat in B)),'texture_size':[512,512],'textures':'Original basecolor, tangent normal, packed ORM PNG; embedded','blender_version':bpy.app.version_string,'modifiers_remaining':sum(len(ob.modifiers) for ob in obs),'source':'Script-modeled original geometry, original baked painted pixels, AI concept reference'}
    print('ASSET_METRICS',name,report[name])
    if tris>20000:raise RuntimeError(name+' exceeds triangle budget '+str(tris))
    bpy.ops.object.select_all(action='DESELECT');root.select_set(True)
    for ob in obs:ob.select_set(True)
    bpy.context.view_layer.objects.active=root
    bpy.ops.export_scene.gltf(filepath=str(OUT/(name+'.glb')),export_format='GLB',use_selection=True,export_yup=True,export_apply=True,export_animations=False,export_texcoords=True,export_normals=True,export_materials='EXPORT',export_image_format='AUTO',export_extras=True)
    for blocks in [bpy.data.materials,bpy.data.images,bpy.data.cameras,bpy.data.lights]:
        for block in list(blocks):
            if block.users==0:blocks.remove(block)
    bpy.ops.wm.save_as_mainfile(filepath=str(SRC/(name+'.blend')))
    report[name]['file_bytes']=(OUT/(name+'.glb')).stat().st_size
    if os.environ.get('SKIP_RENDERS')!='1':
        clear_scene()
        bpy.ops.import_scene.gltf(filepath=str(OUT/(name+'.glb')))
        render_setup(name)
        report[name]['preview_source']='Independently reimported exported GLB'
(ROOT/'docs/art/buildings-validation.json').write_text(json.dumps(report,indent=2))
print('BUILDINGS_COMPLETE')
