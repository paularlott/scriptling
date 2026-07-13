import shutil
import tempfile
import os
import os.path

d = tempfile.mkdtemp()
os.makedirs(d + "/src")
os.write_file(d + "/src/a.txt", "hello")
os.write_file(d + "/src/b.txt", "world")

# copy file
shutil.copy(d + "/src/a.txt", d + "/copy.txt")
assert os.path.exists(d + "/copy.txt"), "copy should create file"

# copytree
shutil.copytree(d + "/src", d + "/backup")
assert os.path.exists(d + "/backup/a.txt"), "copytree should copy contents"
assert os.path.exists(d + "/backup/b.txt"), "copytree should copy contents"

# move
shutil.move(d + "/copy.txt", d + "/moved.txt")
assert os.path.exists(d + "/moved.txt"), "move should create destination"
assert not os.path.exists(d + "/copy.txt"), "move should remove source"

# rmtree
shutil.rmtree(d + "/backup")
assert not os.path.exists(d + "/backup"), "rmtree should remove directory"

# disk_usage
du = shutil.disk_usage(d)
assert du["total"] > 0, "disk total should be positive"
assert du["free"] > 0, "disk free should be positive"

shutil.rmtree(d)
True
