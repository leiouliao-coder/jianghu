"""Hewan village: original old tree, exact fence, five individually placeable props.
Rebuild: python assets/source/props/props_textures.py
         blender --background --python assets/source/props/generate_props.py
All textures are original authored PNGs. Blender 4.3.2. Units metres, front -Y.
"""
import bpy, math, random, json, struct, sys
from pathlib import Path
from mathutils import Vector
R=Path(__file__).resolve().parents[3]
SRC=Path(__file__).resolve().parent; TEX=SRC/'textures'; OUT=R/'client/assets/art/village'; ART=SRC/'renders'; ART.mkdir(exist_ok=True)
random.seed(12349)
bpy.ops.object.select_all(action='SELECT'); bpy.ops.object.delete(use_global=False)
for d in bpy.data.materials: bpy.data.materials.remove(d)
bpy.context.scene.unit_settings.system='METRIC'; bpy.context.scene.unit_settings.scale_length=1.0
mats={}
def material(name,texture=None,color=(.5,.5,.5,1),metal=0,alpha=False):
    m=bpy.data.materials.new(name);m.use_nodes=True;p=m.node_tree.nodes.get('Principled BSDF');p.inputs['Base Color'].default_value=color;p.inputs['Roughness'].default_value=.85;p.inputs['Metallic'].default_value=metal
    if texture:
        n=m.node_tree.nodes.new('ShaderNodeTexImage');n.image=bpy.data.images.load(str(TEX/f'{texture}_albedo.png'),check_existing=True);m.node_tree.links.new(n.outputs['Color'],p.inputs['Base Color'])
        if alpha:
            m.node_tree.links.new(n.outputs['Alpha'],p.inputs['Alpha']);m.surface_render_method='DITHERED';m.use_transparency_overlap=False;m.use_backface_culling=False
        else:
            n=m.node_tree.nodes.new('ShaderNodeTexImage');n.image=bpy.data.images.load(str(TEX/f'{texture}_roughness.png'),check_existing=True);n.image.colorspace_settings.name='Non-Color';m.node_tree.links.new(n.outputs['Color'],p.inputs['Roughness'])
            n=m.node_tree.nodes.new('ShaderNodeTexImage');n.image=bpy.data.images.load(str(TEX/f'{texture}_normal.png'),check_existing=True);n.image.colorspace_settings.name='Non-Color';nm=m.node_tree.nodes.new('ShaderNodeNormalMap');nm.inputs['Strength'].default_value=.5;m.node_tree.links.new(n.outputs['Color'],nm.inputs['Color']);m.node_tree.links.new(nm.outputs['Normal'],p.inputs['Normal'])
    mats[name]=m;return m
wood=material('Weathered umber timber','wood');lightwood=material('Worn timber highlights','wood_light');bark=material('Ancient fluted bark','bark');stone=material('Warm limestone','stone');metal=material('Dark forged iron','metal',metal=.55);cloth=material('Muted jade woven cloth','cloth');paper=material('Cinnabar paper lantern','paper');rope=material('Natural hemp rope','rope');ceramic=material('Ochre glazed earthenware','ceramic');water=material('Still deep jade water',color=(.045,.1,.085,1));black=material('Well interior dark stone',color=(.065,.075,.065,1));gold=material('Worn brass trim',color=(.28,.22,.09,1),metal=.45)
leaves=[material(f'Leaf clusters {i}',f'leaves_{i}',alpha=True) for i in range(4)]
leafmasses=[material(f'Painted foliage mass {i}',f'leaf_mass_{i}') for i in range(4)]

def mesh(name,verts,faces,mat,uvs=None):
    me=bpy.data.meshes.new(name);me.from_pydata(verts,[],faces);me.materials.append(mat);me.update();ob=bpy.data.objects.new(name,me);bpy.context.collection.objects.link(ob)
    uv=me.uv_layers.new(name='UVMap')
    for poly in me.polygons:
        coords=uvs[poly.index] if uvs else [(0,0),(1,0),(1,1),(0,1)]
        for j,li in enumerate(poly.loop_indices):uv.data[li].uv=coords[j%len(coords)]
    return ob

def apply(ob):
    bpy.context.view_layer.objects.active=ob;ob.select_set(True)
    for md in list(ob.modifiers):bpy.ops.object.modifier_apply(modifier=md.name)
    ob.select_set(False)

def box(name,loc,dim,mat,bevel=.015,rot=0):
    bpy.ops.mesh.primitive_cube_add(size=1,location=loc);ob=bpy.context.object;ob.name=name;ob.dimensions=dim;bpy.ops.object.transform_apply(location=False,rotation=False,scale=True);ob.rotation_euler[2]=rot;ob.data.materials.append(mat)
    if bevel:
        md=ob.modifiers.new('Small hand-worn arris','BEVEL');md.width=bevel;md.segments=1;apply(ob)
    ob.select_set(False);return ob

def cyl(name,loc,r,depth,mat,vertices=12,rotation=None,r2=None):
    bpy.ops.mesh.primitive_cone_add(vertices=vertices,radius1=r,radius2=r if r2 is None else r2,depth=depth,location=loc,rotation=rotation or (0,0,0));ob=bpy.context.object;ob.name=name;ob.data.materials.append(mat);ob.select_set(False);return ob

def bar(name,a,b,width,mat,depth=None,bevel=.01):
    a=Vector(a);b=Vector(b);ob=box(name,(a+b)/2,(width,depth or width,(b-a).length),mat,bevel);ob.rotation_euler=(b-a).to_track_quat('Z','Y').to_euler();return ob

def tube(name,pts,radii,mat,sides=8,rough=.0,caps=True):
    verts=[];faces=[];uvs=[]
    for j,(p,r) in enumerate(zip(pts,radii)):
        p=Vector(p);d=Vector(pts[min(j+1,len(pts)-1)])-Vector(pts[max(j-1,0)]);d.normalize();u=d.cross(Vector((0,1,0)))
        if u.length<.01:u=d.cross(Vector((1,0,0)))
        u.normalize();v=d.cross(u).normalized()
        for i in range(sides):
            ang=2*math.pi*i/sides;rr=r*(1+rough*math.sin(i*2.7+j*.7));verts.append(tuple(p+rr*(math.cos(ang)*u+math.sin(ang)*v)))
    for j in range(len(pts)-1):
        for i in range(sides):
            faces.append((j*sides+i,j*sides+(i+1)%sides,(j+1)*sides+(i+1)%sides,(j+1)*sides+i));uvs.append([(i/sides,j/(len(pts)-1)),((i+1)/sides,j/(len(pts)-1)),((i+1)/sides,(j+1)/(len(pts)-1)),(i/sides,(j+1)/(len(pts)-1))])
    if caps:
        faces += [tuple(reversed(range(sides))),tuple((len(pts)-1)*sides+i for i in range(sides))];uvs += [[(.5+.45*math.cos(i*2*math.pi/sides),.5+.45*math.sin(i*2*math.pi/sides)) for i in reversed(range(sides))],[.5]*0+[(.5+.45*math.cos(i*2*math.pi/sides),.5+.45*math.sin(i*2*math.pi/sides)) for i in range(sides)]]
    return mesh(name,verts,faces,mat,uvs)

def lathe(name,rings,mat,sides=16,caps=True):
    pts=[(0,0,z) for z,r in rings];return tube(name,pts,[r for z,r in rings],mat,sides,0,caps)

def ring(name,z,rin,rout,h,mat,segs=20):
    verts=[];faces=[]
    for zz in [z-h/2,z+h/2]:
        for rr in [rin,rout]:
            verts.extend([(rr*math.cos(i*math.tau/segs),rr*math.sin(i*math.tau/segs),zz) for i in range(segs)])
    for i in range(segs):
        j=(i+1)%segs
        faces.extend([(i,j,segs+j,segs+i),(2*segs+i,3*segs+i,3*segs+j,2*segs+j),(i,2*segs+i,2*segs+j,j),(segs+i,segs+j,3*segs+j,3*segs+i)])
    return mesh(name,verts,faces,mat)

def join(parts,name):
    bpy.ops.object.select_all(action='DESELECT')
    for o in parts:o.select_set(True)
    bpy.context.view_layer.objects.active=parts[0];bpy.ops.object.join();ob=bpy.context.object;ob.name=name;bpy.context.scene.cursor.location=(0,0,0);bpy.ops.object.origin_set(type='ORIGIN_CURSOR');bpy.ops.object.transform_apply(location=False,rotation=True,scale=True);ob.select_set(False);return ob

# OLD TREE: buttressed ground-safe roots, twisting fluted trunk, open branch architecture.
parts=[]
parts.append(tube('Twisting old trunk',[(0,0,.02),(.08,.04,.34),(-.09,.06,.9),(-.03,.09,1.6),(.18,.07,2.25),(.28,.12,2.9),(.08,.22,3.6),(-.3,.35,4.28),(-.74,.4,4.9),(-1.05,.52,5.55)],[.8,.76,.68,.61,.59,.51,.42,.3,.13,.018],bark,10,.15))
for i in range(8):
    ang=i*math.tau/8+.2;vx,vy=math.cos(ang),math.sin(ang)
    parts.append(tube('Ground buttress root',[(vx*.35,vy*.35,.8),(vx*.74,vy*.74,.27),(vx*1.12,vy*1.12,.1),(vx*1.37,vy*1.37,.04)],[.31,.26,.14,.045],bark,6,.12))
clusters=[]
for i in range(10):
    a=i*math.tau/10+.3;dist=3.8+random.uniform(-.55,.65);zz=4.8+random.uniform(-.35,1.3)
    p0=(.1,.15,2.2+(i%3)*.52);p1=(math.cos(a)*1.2,math.sin(a)*1.2,3.2+(i%3)*.55);p2=(math.cos(a+.1)*2.6,math.sin(a+.1)*2.6,zz-.25);p3=(math.cos(a)*dist,math.sin(a)*dist,zz)
    parts.append(tube('Reaching main bough',[p0,p1,p2,p3],[.37,.27,.145,.045],bark,7,.12))
    clusters.append((Vector(p3),random.uniform(1.35,1.8),random.uniform(.68,.95)))
    for j in range(2):
        s=-1 if j==0 else 1;aa=a+s*.36;pt=(math.cos(aa)*(dist+.12),math.sin(aa)*(dist+.12),zz+.5+.2*j)
        parts.append(tube('Forking upper twig',[p2,((p2[0]+pt[0])*.5,(p2[1]+pt[1])*.5,zz+.15),pt],[.12,.06,.012],bark,5))
        clusters.append((Vector(pt),random.uniform(.95,1.35),.7))
clusters.extend([(Vector((-.55,.3,6.2)),2.0,.9),(Vector((1.1,.5,6.2)),1.65,.8),(Vector((-.7,-1.1,5.9)),1.7,.8),(Vector((1.4,-1.4,6.0)),1.7,.8),(Vector((-1.6,1.4,6.1)),1.7,.8),(Vector((-1.8,-1.4,5.8)),1.7,.8),(Vector((1.5,1.6,6.2)),1.7,.8)])
# Hand-shaped crown volumes with a restrained alpha-cutout leaf fringe.
leafverts=[[] for _ in leaves];leaffaces=[[] for _ in leaves];leafuv=[[] for _ in leaves]
clusters=[cl for i,cl in enumerate(clusters) if i>=30 or i%3!=2]
for ci,(center,rx,rz) in enumerate(clusters):
    # Hand-shaped irregular oblate crown volumes, each with only 48 triangles.
    # Surface brushwork and a tiny alpha fringe provide detail without a stack of flat cards.
    vv=[tuple(center+Vector((0,0,-rz*.8)))];ff=[];uu=[]
    for row,zz in enumerate([-.65,0,.65]):
        for k in range(8):
            aa=k*math.tau/8;rad=rx*([.71,1,.71][row]+.05*math.sin(k*2.1+ci))
            vv.append(tuple(center+Vector((math.cos(aa)*rad,math.sin(aa)*rad*(.9+.06*math.sin(ci)),rz*(zz+.1*math.sin(k*1.7+ci))))))
    vv.append(tuple(center+Vector((rx*.08*math.sin(ci),rx*.08*math.cos(ci),rz*1.03))))
    for k in range(8):
        j=(k+1)%8;u0=k/8;u1=(k+1)/8
        ff.append((0,1+j,1+k));uu.append([((u0+u1)/2,0),(u1,.3),(u0,.3)])
        ff.append((1+k,1+j,9+j,9+k));uu.append([(u0,.25),(u1,.25),(u1,.5),(u0,.5)])
        ff.append((9+k,9+j,17+j,17+k));uu.append([(u0,.5),(u1,.5),(u1,.75),(u0,.75)])
        ff.append((17+k,17+j,25));uu.append([(u0,.75),(u1,.75),((u0+u1)/2,1)])
    uu=[[(u*4,v*2) for u,v in face] for face in uu]
    parts.append(mesh('Sculpted painted crown cluster',vv,ff,leafmasses[ci%4],uu))
    for k in range(2):
        angle=k*2.39996+ci;rr=.95;pp=center+Vector((math.cos(angle)*rx*rr,math.sin(angle)*rx*rr,rz*math.sqrt(1-rr*rr)+random.uniform(-.35,-.1)))
        scale=random.uniform(.75,1.05);variant=(ci+k)%4
        # Orient a broad spray mainly upward, with irregular tilts preserving a soft silhouette.
        theta=random.uniform(0,math.tau);tilt=random.uniform(.12,.8)
        u=Vector((math.cos(theta),math.sin(theta),0));v=Vector((-math.sin(theta)*math.cos(tilt),math.cos(theta)*math.cos(tilt),math.sin(tilt)))
        vv=leafverts[variant];ff=leaffaces[variant];uv=leafuv[variant];base=len(vv)
        for x,y in [(-1,-1),(1,-1),(1,1),(-1,1)]:vv.append(tuple(pp+u*x*scale*.65+v*y*scale*.65))
        ff.append(tuple(base+i for i in range(4)));uv.append([(0,0),(1,0),(1,1),(0,1)])
for i in range(4):parts.append(mesh(f'Painted leaf sprays {i}',leafverts[i],leaffaces[i],leaves[i],leafuv[i]))
tree=join(parts,'tree_old');tree['collision_trunk_radius_m']=1.5;tree['canopy_blocks_movement']=False;tree['front']='-Y';tree['design']='Old locust with buttressed, split-reaching trunk and hand-painted alpha-cutout sprays'
# Bring crown to an exact 12.0 m widest span without changing collision trunk.
# Leaf / branch proportions were explicitly authored in metres; actual canopy is recorded below.

# FENCE: precise outer envelope 4 x .3 x 1.2 m, internal mortise structure.
parts=[]
for x in [-1.89,0,1.89]:
    parts += [box('Aged upright',(x,0,.585),(.22,.25,1.17),wood,.016),box('Worn post cap',(x,0,1.175),(.22,.3,.05),lightwood,.012)]
for z in [.37,.87]:parts.append(box('Long mortised rail',(0,0,z),(4,.17,.16),wood,.014))
for i in range(10):
    x=-1.65+i*(3.3/9);parts.append(box('Irregular narrow stave',(x,.025,.62),(.085,.115,.82+random.uniform(-.04,.04)),lightwood,.009,random.uniform(-.018,.018)))
parts.append(bar('Western repair brace',(-1.76,-.105,.46),(-.12,-.105,.80),.09,wood,.05,.006));parts.append(bar('Eastern repair brace',(.12,-.105,.46),(1.76,-.105,.80),.09,wood,.05,.006))
for x in [-1.89,0,1.89]:
    for z in [.37,.87]:parts.append(cyl('Square-headed iron pin',(x,-.092,z),.024,.026,metal,6,(math.pi/2,0,0)))
fence=join(parts,'fence_wood_4m');fence['footprint_m']='4.0 x 0.3';fence['height_m']=1.2

# CRATE: inset plank faces, individually chamfered braces and flat iron nail heads.
parts=[]
for i in range(5):
    z=.13+i*.17
    for y in [-.4,.4]:parts.append(box('Crate face plank',(0,y,z),(1.02,.075,.157),lightwood,.01))
    for x in [-.49,.49]:parts.append(box('Crate side plank',(x,0,z),(.075,.8,.157),wood,.008))
for i in range(5):parts.append(box('Crate lid plank',(-.4+i*.2,0,.91),(.187,.85,.07),lightwood,.009))
for x in [-.43,.43]:
    for y in [-.445,.445]:parts.append(box('Crate upright binding',(x,y,.48),(.09,.075,.94),wood,.01))
parts += [bar('Front diagonal brace',(-.39,-.49,.14),(.4,-.49,.81),.095,wood,.045,.008),bar('Back diagonal brace',(.39,.49,.14),(-.4,.49,.81),.095,wood,.045,.008)]
for x in [-.43,.43]:
    for z in [.12,.84]:parts.append(cyl('Crate nail',(x,-.489,z),.019,.012,metal,6,(math.pi/2,0,0)))
crate=join(parts,'crate');

# BARREL: twelve separate curved staves with visible seams, forged hoops, inset timber lid.
parts=[]
N=14
for k in range(N):
    verts=[];faces=[];a=k*math.tau/N+.012;b=(k+1)*math.tau/N-.012;rings=[(.04,.34),(.15,.38),(.48,.43),(.81,.39),(.94,.34)]
    for z,r in rings:
        for ang in [a,b]:verts.append((r*math.cos(ang),r*math.sin(ang),z))
    for j in range(len(rings)-1):faces.append((j*2,j*2+1,j*2+3,j*2+2))
    parts.append(mesh('Curved separate barrel stave',verts,faces,lightwood if k%4==0 else wood))
for z,r in [(.15,.383),(.76,.405),(.90,.355)]:parts.append(ring('Forged iron hoop',z,r-.005,r+.014,.055,metal,N))
parts.append(cyl('Inset stave lid',(0,0,.925),.326,.022,lightwood,14))
for x in [-.2,-.07,.07,.2]:
    length=2*math.sqrt(.32*.32-x*x);parts.append(box('Lid seam',(x,0,.939),(.008,length,.003),wood,0))
parts.append(cyl('Barrel bung',(0,.1,.949),.04,.014,wood,10))
barrel=join(parts,'barrel')

# LANTERN POLE: restrained late-imperial vernacular paper light with brass rim and tassel.
parts=[]
parts.append(box('Lantern stone foot',(0,0,.1),(.44,.44,.2),stone,.045));parts.append(tube('Slightly crooked lamp post',[(0,0,.17),(-.035,0,1.05),(.025,0,2.15),(0,0,2.7)],[.075,.068,.061,.053],wood,8,.06));parts.append(bar('Lamp crossarm',(-.13,0,2.6),(.69,0,2.6),.09,wood,.1));parts.append(bar('Lamp diagonal brace',(.015,0,2.15),(.47,0,2.58),.055,lightwood,.055));parts.append(cyl('Lantern hanging loop',(.53,0,2.46),.012,.27,metal,6))
lan=lathe('Faceted paper lantern',[(1.76,.15),(1.83,.23),(2.08,.27),(2.29,.22),(2.35,.15)],paper,12,True);lan.location.x=.53;parts.append(lan)
for z,r in [(1.76,.17),(2.35,.17)]:
    o=cyl('Lantern brass rim',(.53,0,z),r,.035,gold,12);parts.append(o)
for i in range(8):
    a=i*math.tau/8
    parts.append(tube('Fine lantern rib',[(.53+math.cos(a)*.15,math.sin(a)*.15,1.77),(.53+math.cos(a)*.268,math.sin(a)*.268,2.07),(.53+math.cos(a)*.15,math.sin(a)*.15,2.34)],[.005]*3,gold,4))
parts.append(tube('Lantern tassel cord',[(.53,0,1.75),(.53,0,1.61)],[.012,.012],rope,6));parts.append(cyl('Cinnabar tassel',(.53,0,1.53),.039,.16,paper,8,r2=.018));lantern=join(parts,'lantern_pole')
# Recenter actual asymmetrical lantern footprint while preserving ground plane.
for v in lantern.data.vertices:v.co.x-=.275

# WELL: true open masonry ring, staggered dressed stones, hand-crank and bucket.
parts=[]
for row in range(2):
    for k in range(12):
        a=k*math.tau/12+(row%2)*math.pi/12+.008;b=a+math.tau/12-.016;lo=.04+row*.27;hi=lo+.255;ri=.56;ro=.87
        verts=[(r*math.cos(t),r*math.sin(t),z) for z in [lo,hi] for r in [ri,ro] for t in [a,b]]
        ob=mesh('Individual ring stone',verts,[(0,2,3,1),(4,5,7,6),(0,1,5,4),(2,6,7,3),(0,4,6,2),(1,3,7,5)],stone);md=ob.modifiers.new('Soft dressed stone edges','BEVEL');md.width=.018;md.segments=1;apply(ob);parts.append(ob)
for k in range(12):
    a=k*math.tau/12+.01;b=a+math.tau/12-.02;verts=[(r*math.cos(t),r*math.sin(t),z) for z in [.58,.72] for r in [.53,.93] for t in [a,b]]
    ob=mesh('Broad worn coping stone',verts,[(0,2,3,1),(4,5,7,6),(0,1,5,4),(2,6,7,3),(0,4,6,2),(1,3,7,5)],stone);md=ob.modifiers.new('Worn coping corners','BEVEL');md.width=.025;md.segments=1;apply(ob);parts.append(ob)
parts.append(cyl('Opaque recessed well water',(0,0,.1),.55,.02,water,24))
for x in [-.76,.76]:parts.append(box('Well timber upright',(x,0,1.12),(.13,.17,2.16),wood,.018));parts.append(box('Upright stone socket',(x,0,.14),(.25,.28,.28),stone,.025))
parts.append(box('Well top lintel',(0,0,2.2),(1.82,.24,.16),wood,.025));parts.append(cyl('Well winch drum',(0,0,1.38),.10,1.67,lightwood,12,(0,math.pi/2,0)))
parts.append(bar('Winch crank',(.89,0,1.38),(.89,0,1.65),.045,metal));parts.append(cyl('Crank timber hand grip',(.91,0,1.65),.045,.17,wood,8,(0,math.pi/2,0)))
for k in range(8):
    # Rope winding presented as eight real loops around the central drum.
    o=ring('Rope winding',0,.105,.118,.022,rope,10);o.rotation_euler[1]=math.pi/2;o.location=(k*.024-.084,0,1.38);parts.append(o)
parts.append(tube('Hanging hemp line',[(.01,-.12,1.38),(.01,-.12,1.0),(.01,-.12,.76)],[.014]*3,rope,6))
bucket=lathe('Small wooden bucket',[(.73,.11),(.92,.15)],wood,10,False);bucket.location=(.01,-.12,0);parts.append(bucket)
for z in [.76,.89]:o=ring('Bucket iron band',z,.12,.135,.025,metal,10);o.location.y=-.12;parts.append(o)
parts.append(bar('Bucket handle',(-.11,-.12,.92),(.13,-.12,.92),.025,metal));well=join(parts,'well')

# MARKET TABLE: sturdy trestles, cloth drape, shallow wicker trays, restrained wares.
parts=[]
for x in [-.68,.68]:
    for y in [-.31,.31]:parts.append(bar('Splayed market table leg',(x,y,.03),(x*.94,y*.85,.88),.10,wood,.10,.012))
    parts.append(box('Table trestle foot',(x,0,.09),(.14,.85,.11),wood,.014))
parts.append(box('Long lower stretcher',(0,0,.30),(1.45,.10,.12),lightwood,.012))
for y in [-.34,.34]:parts.append(box('Table apron',(0,y,.79),(1.53,.10,.16),wood,.012))
for j in range(6):parts.append(box('Long weathered table plank',(0,-.45+j*.18,.91),(1.88,.166,.10),lightwood,.011))
# cloth drape made as a shaped quad strip; no simulation required.
verts=[];xs=[-.55,-.30,-.05];ys=[-.58,-.49,-.30,.0,.30,.49,.55];zs=[.57,.975,.977,.977,.977,.96,.73]
for x in xs:
    for y,z in zip(ys,zs):verts.append((x,y,z+.008*math.sin(x*21+y*7)))
faces=[]
for i in range(2):
    for j in range(6):faces.append((i*7+j,(i+1)*7+j,(i+1)*7+j+1,i*7+j+1))
parts.append(mesh('Jade linen table runner',verts,faces,cloth))
for x,y in [(.4,.16),(.46,-.27)]:
    o=lathe('Low woven produce tray',[(.97,.16),(1.01,.20),(1.06,.21)],rope,12,False);o.location.x=x;o.location.y=y;parts.append(o)
    for k in range(5):
        a=k*math.tau/5;rr=.09;cx=x+math.cos(a)*rr;cy=y+math.sin(a)*rr
        cv=[];cf=[]
        for zz,rad in [(1.025,.033),(1.125,.055),(1.125,.043),(1.04,.026)]:
            cv.extend([(cx+rad*math.cos(t*math.tau/8),cy+rad*math.sin(t*math.tau/8),zz) for t in range(8)])
        for j in range(3):
            for t in range(8):cf.append((j*8+t,j*8+(t+1)%8,(j+1)*8+(t+1)%8,(j+1)*8+t))
        cf.extend([tuple(reversed(range(8))),tuple(range(24,32))]);parts.append(mesh('Hollow earthenware tea cup',cv,cf,ceramic))
# two folded bolts of cloth suggest village commerce without overcrowding the silhouette.
parts.append(box('Folded cloth bundle',(-.27,.14,1.035),(.42,.30,.105),cloth,.025));parts.append(box('Ochre folded sack',(-.28,.14,1.12),(.34,.26,.065),rope,.018))
table=join(parts,'market_table')
props=[crate,barrel,lantern,well,table]
for o in [tree,fence]+props:o['front']='-Y';o['origin']='footprint center at ground';o['authorship']='Original deterministic mesh construction and hand-painted PNG textures; no third-party assets'
# Place exact lower bound at ground; center each small prop footprint around its own local origin.
for o in props:
    vs=[v.co for v in o.data.vertices];mn=Vector(tuple(min(v[i] for v in vs) for i in range(3)));mx=Vector(tuple(max(v[i] for v in vs) for i in range(3)));center=Vector(((mn.x+mx.x)/2,(mn.y+mx.y)/2,mn.z))
    for v in o.data.vertices:v.co-=center

# Smooth only organic geometry; furniture keeps clean deliberate facets.
for p in tree.data.polygons:p.use_smooth=True
# Alpha cutout is patched to MASK after export; Principled image + Alpha remains engine portable.
def alpha_mask_glb(path):
    data=path.read_bytes();magic,ver,size=struct.unpack_from('<III',data,0);ln,typ=struct.unpack_from('<II',data,12);j=json.loads(data[20:20+ln]);rest=data[20+ln:]
    for m in j.get('materials',[]):
        if m.get('name','').startswith('Leaf clusters'):m['alphaMode']='MASK';m['alphaCutoff']=.42;m['doubleSided']=True
    js=json.dumps(j,separators=(',',':')).encode();js+=b' ' *((-len(js))%4);path.write_bytes(struct.pack('<III',magic,ver,12+8+len(js)+len(rest))+struct.pack('<II',len(js),typ)+js+rest)

def export(path,objects):
    bpy.ops.object.select_all(action='DESELECT')
    for o in objects:o.select_set(True)
    bpy.context.view_layer.objects.active=objects[0]
    bpy.ops.export_scene.gltf(filepath=str(path),export_format='GLB',use_selection=True,export_yup=True,export_apply=True,export_texcoords=True,export_normals=True,export_materials='EXPORT',export_animations=False,export_extras=True)
    alpha_mask_glb(path)

def metric(o):
    me=o.data;me.calc_loop_triangles();vs=[v.co for v in me.vertices];mn=[min(v[i] for v in vs) for i in range(3)];mx=[max(v[i] for v in vs) for i in range(3)]
    return {'name':o.name,'triangles':len(me.loop_triangles),'vertices':len(me.vertices),'local_min_m':mn,'local_max_m':mx,'dimensions_m':[mx[i]-mn[i] for i in range(3)],'origin_local_m':[0,0,0],'display_translation_blender_m':list(o.location),'front':'-Y'}
# Normalize only tree XY to 12 m canopy span, preserving trunk/roots (already under r=1.5).
# Vertical geometry remains an entirely authored silhouette.
xs=[v.co.x for v in tree.data.vertices];ys=[v.co.y for v in tree.data.vertices];can=max(max(xs)-min(xs),max(ys)-min(ys));factor=12/can
for v in tree.data.vertices:
    if v.co.z>2.5:v.co.x*=factor;v.co.y*=factor
for v in tree.data.vertices:v.co.z=max(0,v.co.z)
export(OUT/'tree_old.glb',[tree]);export(OUT/'fence_wood_4m.glb',[fence])
arrangement=[(-3.0,0,0),(-1.5,0,0),(0,0,0),(2.0,0,0),(0,2.0,0)]
for o,loc in zip(props,arrangement):o.location=loc
export(OUT/'props_set.glb',props)
metrics={'blender_version':bpy.app.version_string,'generation':'Original authored SVG design, deterministic Blender Python mesh construction, original hand-painted procedural PNG textures','texture_dimensions':{im.name:list(im.size) for im in bpy.data.images if im.source=='FILE'},'tree_old.glb':metric(tree),'fence_wood_4m.glb':metric(fence),'props_set.glb':{'objects':[metric(o) for o in props],'arrangement':'Five separate root mesh objects. Each local origin is footprint center at ground. Collection translations spread the props for inspection; reset a root translation when instancing it individually.'},'technical_notes':['Leaf-fringe materials use glTF MASK alpha cutout at 0.42, two-sided, original PNG albedo; sculpted leaf masses are opaque','No transmission, subsurface, Blender-only procedural textures, external asset dependencies, or unapplied modifiers','Only tree trunk collision radius 1.5 m; branches and canopy are visual decoration','Blender +Z up converted on export to glTF +Y up']}
for f in ['tree_old.glb','fence_wood_4m.glb','props_set.glb']:metrics[f]['file_bytes']=(OUT/f).stat().st_size
(SRC/'props-metrics.json').write_text(json.dumps(metrics,indent=2))
# Packed editable source. The authored display layout is kept in the file with a studio render setup.
tree.location=(0,7,0);fence.location=(0,3,0)
for o in props:o.location.y-=2
for im in bpy.data.images:
    if im.source=='FILE':im.pack()
scene=bpy.context.scene;scene.render.engine='CYCLES';scene.cycles.samples=64;scene.cycles.use_denoising=False;scene.cycles.transparent_max_bounces=64
scene.world.color=(.35,.35,.35)
world=scene.world;world.use_nodes=True;world.node_tree.nodes.get('Background').inputs['Color'].default_value=(.38,.43,.40,1);world.node_tree.nodes.get('Background').inputs['Strength'].default_value=.65
floor=box('STUDIO ONLY · ground',(0,3,-.07),(200,200,.1),material('Studio warm ground',color=(.24,.26,.22,1)),0)
for name,loc,energy,size in [('Warm key',(-7,-7,15),1800,9),('Soft fill',(7,3,11),1300,8)]:
    bpy.ops.object.light_add(type='AREA',location=loc);li=bpy.context.object;li.name=name;li.data.energy=energy;li.data.shape='DISK';li.data.size=size;li.rotation_euler=(Vector((0,3,1))-li.location).to_track_quat('-Z','Y').to_euler()
bpy.ops.object.light_add(type='SUN',location=(0,0,10));sun=bpy.context.object;sun.rotation_euler=(.4,-.5,-.7);sun.data.energy=1.8;sun.data.angle=.3
bpy.ops.object.camera_add(location=(15,-20,18));cam=bpy.context.object;cam.name='Asset overview camera';cam.rotation_euler=(Vector((0,3,2.6))-cam.location).to_track_quat('-Z','Y').to_euler();cam.data.type='ORTHO';cam.data.ortho_scale=22;scene.camera=cam
scene.render.resolution_x=1600;scene.render.resolution_y=1200;scene.render.resolution_percentage=100;scene.view_settings.view_transform='AgX'
bpy.ops.wm.save_as_mainfile(filepath=str(SRC/'props.blend'))
scene.render.filepath=str(ART/'props-assets-overview.png');bpy.ops.render.render(write_still=True)
# Close detail sheet using actual small asset geometry.
if '--overview-only' not in sys.argv:
    tree.hide_render=True;fence.hide_render=True;cam.location=(8,-11,9);cam.rotation_euler=(Vector((-.1,-1.25,1))-cam.location).to_track_quat('-Z','Y').to_euler();cam.data.ortho_scale=10.2;scene.render.resolution_x=1800;scene.render.resolution_y=1100;scene.render.filepath=str(ART/'props-assets-detail.png');bpy.ops.render.render(write_still=True)
print('HEWAN PROP BUILD COMPLETE',json.dumps({k:metrics[k] for k in ['tree_old.glb','fence_wood_4m.glb']},indent=2))
