include $(GNUSTEP_MAKEFILES)/common.make

LIBRARY_NAME = libadder
libadder_OBJC_FILES = Adder.m
libadder_HEADER_FILES = Adder.h

TOOL_NAME = add_test
add_test_OBJC_FILES = add_test.m
add_test_LIB_DIRS = -L./obj
add_test_TOOL_LIBS = -ladder

include $(GNUSTEP_MAKEFILES)/library.make
include $(GNUSTEP_MAKEFILES)/tool.make

check:: all
	LD_LIBRARY_PATH=./obj ./obj/add_test
