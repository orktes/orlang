@0 = constant [4 x i8] c"%d\0A\00"
@1 = constant [4 x i8] c"%f\0A\00"
@2 = constant [4 x i8] c"%s\0A\00"
@3 = constant [4 x i8] c"%d\0A\00"
@"__itable_int32_to_interace anything {  }" = constant { i32, [0 x i8*] } { i32 1, [0 x i8*] [] }
@"__itable_float32_to_interace anything {  }" = constant { i32, [0 x i8*] } { i32 2, [0 x i8*] [] }
@4 = constant [7 x i8] c"string\00"
@"__itable_string_to_interace anything {  }" = constant { i32, [0 x i8*] } { i32 3, [0 x i8*] [] }
@"__itable_bool_to_interace anything {  }" = constant { i32, [0 x i8*] } { i32 4, [0 x i8*] [] }

declare i64 @printf(i8* %format, ...)

define void @print({ i8*, i8* } %v) {
0:
	%1 = alloca { i8*, i8* }
	store { i8*, i8* } %v, { i8*, i8* }* %1
	%2 = load { i8*, i8* }, { i8*, i8* }* %1
	%3 = extractvalue { i8*, i8* } %2, 1
	%4 = bitcast i8* %3 to { i32, [0 x i8*] }*
	%5 = getelementptr { i32, [0 x i8*] }, { i32, [0 x i8*] }* %4, i32 0, i32 0
	%6 = load i32, i32* %5
	%7 = icmp eq i32 %6, 1
	br i1 %7, label %8, label %12

8:
	%9 = load { i8*, i8* }, { i8*, i8* }* %1
	%10 = call i64 (i8*, ...) @printf(i8* getelementptr ([4 x i8], [4 x i8]* @0, i32 0, i32 0), { i8*, i8* } %9)
	br label %11

11:
	ret void

12:
	%13 = load { i8*, i8* }, { i8*, i8* }* %1
	%14 = extractvalue { i8*, i8* } %13, 1
	%15 = bitcast i8* %14 to { i32, [0 x i8*] }*
	%16 = getelementptr { i32, [0 x i8*] }, { i32, [0 x i8*] }* %15, i32 0, i32 0
	%17 = load i32, i32* %16
	%18 = icmp eq i32 %17, 2
	br i1 %18, label %19, label %23

19:
	%20 = load { i8*, i8* }, { i8*, i8* }* %1
	%21 = call i64 (i8*, ...) @printf(i8* getelementptr ([4 x i8], [4 x i8]* @1, i32 0, i32 0), { i8*, i8* } %20)
	br label %22

22:
	br label %11

23:
	%24 = load { i8*, i8* }, { i8*, i8* }* %1
	%25 = extractvalue { i8*, i8* } %24, 1
	%26 = bitcast i8* %25 to { i32, [0 x i8*] }*
	%27 = getelementptr { i32, [0 x i8*] }, { i32, [0 x i8*] }* %26, i32 0, i32 0
	%28 = load i32, i32* %27
	%29 = icmp eq i32 %28, 3
	br i1 %29, label %30, label %34

30:
	%31 = load { i8*, i8* }, { i8*, i8* }* %1
	%32 = call i64 (i8*, ...) @printf(i8* getelementptr ([4 x i8], [4 x i8]* @2, i32 0, i32 0), { i8*, i8* } %31)
	br label %33

33:
	br label %22

34:
	%35 = load { i8*, i8* }, { i8*, i8* }* %1
	%36 = extractvalue { i8*, i8* } %35, 1
	%37 = bitcast i8* %36 to { i32, [0 x i8*] }*
	%38 = getelementptr { i32, [0 x i8*] }, { i32, [0 x i8*] }* %37, i32 0, i32 0
	%39 = load i32, i32* %38
	%40 = icmp eq i32 %39, 4
	br i1 %40, label %41, label %44

41:
	%42 = load { i8*, i8* }, { i8*, i8* }* %1
	%43 = call i64 (i8*, ...) @printf(i8* getelementptr ([4 x i8], [4 x i8]* @3, i32 0, i32 0), { i8*, i8* } %42)
	br label %44

44:
	br label %33
}

define i32 @main() {
0:
	%1 = bitcast i32 123 to i8*
	%2 = bitcast { i32, [0 x i8*] }* @"__itable_int32_to_interace anything {  }" to i8*
	%3 = insertvalue { i8*, i8* } undef, i8* %1, 0
	%4 = insertvalue { i8*, i8* } %3, i8* %2, 1
	call void @print({ i8*, i8* } %4)
	%5 = bitcast float 0x405EDD2F00000000 to i8*
	%6 = bitcast { i32, [0 x i8*] }* @"__itable_float32_to_interace anything {  }" to i8*
	%7 = insertvalue { i8*, i8* } undef, i8* %5, 0
	%8 = insertvalue { i8*, i8* } %7, i8* %6, 1
	call void @print({ i8*, i8* } %8)
	%9 = bitcast { i32, [0 x i8*] }* @"__itable_string_to_interace anything {  }" to i8*
	%10 = insertvalue { i8*, i8* } undef, i8* getelementptr ([7 x i8], [7 x i8]* @4, i32 0, i32 0), 0
	%11 = insertvalue { i8*, i8* } %10, i8* %9, 1
	call void @print({ i8*, i8* } %11)
	%12 = bitcast i1 true to i8*
	%13 = bitcast { i32, [0 x i8*] }* @"__itable_bool_to_interace anything {  }" to i8*
	%14 = insertvalue { i8*, i8* } undef, i8* %12, 0
	%15 = insertvalue { i8*, i8* } %14, i8* %13, 1
	call void @print({ i8*, i8* } %15)
	ret i32 0
}
