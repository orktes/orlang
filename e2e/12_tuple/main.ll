@0 = constant [7 x i8] c"string\00"
@1 = constant [31 x i8] c"int: %d, float: %f, string: %s\00"
@2 = constant [8 x i8] c"%d%f%s\0A\00"
@3 = constant [4 x i8] c"%s\0A\00"

declare i8* @malloc(i64 %size)

declare void @free(i8* %ptr)

declare i64 @sprintf(i8* %buf, i8* %format, ...)

declare i64 @printf(i8* %format, ...)

define { i32, float, i8* } @returnTupple() {
0:
	%1 = alloca { i32, float, i8* }
	%2 = getelementptr { i32, float, i8* }, { i32, float, i8* }* %1, i32 0, i32 0
	store i32 1, i32* %2
	%3 = getelementptr { i32, float, i8* }, { i32, float, i8* }* %1, i32 0, i32 1
	store float 0.5, float* %3
	%4 = getelementptr { i32, float, i8* }, { i32, float, i8* }* %1, i32 0, i32 2
	store i8* getelementptr ([7 x i8], [7 x i8]* @0, i32 0, i32 0), i8** %4
	%5 = load { i32, float, i8* }, { i32, float, i8* }* %1
	ret { i32, float, i8* } %5
}

define i8* @receivesTupple({ i32, float, i8* } %receives) {
0:
	%1 = alloca { i32, float, i8* }
	store { i32, float, i8* } %receives, { i32, float, i8* }* %1
	%2 = load { i32, float, i8* }, { i32, float, i8* }* %1
	%3 = extractvalue { i32, float, i8* } %2, 0
	%4 = alloca i32
	store i32 %3, i32* %4
	%5 = extractvalue { i32, float, i8* } %2, 1
	%6 = alloca float
	store float %5, float* %6
	%7 = extractvalue { i32, float, i8* } %2, 2
	%8 = alloca i8*
	store i8* %7, i8** %8
	%9 = sext i32 1024 to i64
	%10 = call i8* @malloc(i64 %9)
	%11 = alloca i8*
	store i8* %10, i8** %11
	%12 = load i8*, i8** %11
	%13 = load i32, i32* %4
	%14 = load float, float* %6
	%15 = fpext float %14 to double
	%16 = load i8*, i8** %8
	%17 = call i64 (i8*, i8*, ...) @sprintf(i8* %12, i8* getelementptr ([31 x i8], [31 x i8]* @1, i32 0, i32 0), i32 %13, double %15, i8* %16)
	%18 = load i8*, i8** %11
	ret i8* %18
}

define i32 @main() {
0:
	%1 = call { i32, float, i8* } @returnTupple()
	%2 = extractvalue { i32, float, i8* } %1, 0
	%3 = alloca i32
	store i32 %2, i32* %3
	%4 = extractvalue { i32, float, i8* } %1, 1
	%5 = alloca float
	store float %4, float* %5
	%6 = extractvalue { i32, float, i8* } %1, 2
	%7 = alloca i8*
	store i8* %6, i8** %7
	%8 = load i32, i32* %3
	%9 = load float, float* %5
	%10 = fpext float %9 to double
	%11 = load i8*, i8** %7
	%12 = call i64 (i8*, ...) @printf(i8* getelementptr ([8 x i8], [8 x i8]* @2, i32 0, i32 0), i32 %8, double %10, i8* %11)
	%13 = load i32, i32* %3
	%14 = load float, float* %5
	%15 = load i8*, i8** %7
	%16 = alloca { i32, float, i8* }
	%17 = getelementptr { i32, float, i8* }, { i32, float, i8* }* %16, i32 0, i32 0
	store i32 %13, i32* %17
	%18 = getelementptr { i32, float, i8* }, { i32, float, i8* }* %16, i32 0, i32 1
	store float %14, float* %18
	%19 = getelementptr { i32, float, i8* }, { i32, float, i8* }* %16, i32 0, i32 2
	store i8* %15, i8** %19
	%20 = load { i32, float, i8* }, { i32, float, i8* }* %16
	%21 = call i8* @receivesTupple({ i32, float, i8* } %20)
	%22 = alloca i8*
	store i8* %21, i8** %22
	%23 = load i8*, i8** %22
	%24 = call i64 (i8*, ...) @printf(i8* getelementptr ([4 x i8], [4 x i8]* @3, i32 0, i32 0), i8* %23)
	%25 = load i8*, i8** %22
	call void @free(i8* %25)
	ret i32 0
}
