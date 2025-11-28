
import pathlib
import os
import os.path

def test_path_creation():
    p = pathlib.Path("foo/bar.txt")
    assert p.__str__ == "foo/bar.txt"
    assert p.name == "bar.txt"
    assert p.parent == "foo"
    assert p.stem == "bar"
    assert p.suffix == ".txt"
    assert p.parts == ("foo", "bar.txt")
    print("Path creation tests passed")

def test_joinpath():
    p = pathlib.Path("foo")
    p2 = p.joinpath("bar", "baz.txt")
    assert p2.__str__ == "foo/bar/baz.txt"
    print("Joinpath tests passed")

def test_file_ops():
    # Setup
    test_dir = "test_pathlib_dir"
    test_file = "test_pathlib_dir/test.txt"

    print("Cleaning up...")
    if os.path.exists(test_dir):
        # import shutil # shutil not implemented yet
        # shutil not implemented yet, use os
        if os.path.isdir(test_dir):
            # recursive delete not implemented in minimal os, so we do simple cleanup
            if os.path.exists(test_file):
                print("Removing file...")
                os.remove(test_file)
            print("Removing dir...")
            os.rmdir(test_dir)

    print("Creating dir...")
    # Test mkdir
    p_dir = pathlib.Path(test_dir)
    p_dir.mkdir()
    assert p_dir.exists()
    assert p_dir.is_dir()

    print("Writing file...")
    # Test write_text
    p_file = p_dir.joinpath("test.txt")
    p_file.write_text("hello world")
    assert p_file.exists()
    assert p_file.is_file()

    print("Reading file...")
    # Test read_text
    content = p_file.read_text()
    assert content == "hello world"

    print("Unlinking...")
    # Test unlink
    p_file.unlink()
    assert not p_file.exists()

    print("Removing dir (pathlib)...")
    # Test rmdir
    p_dir.rmdir()
    assert not p_dir.exists()

    print("File operations tests passed")

def main():
    print("Running pathlib tests...")
    test_path_creation()
    test_joinpath()
    test_file_ops()
    print("All pathlib tests passed!")

main()
