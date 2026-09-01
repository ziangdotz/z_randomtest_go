#ifndef FUNC_NIST_H
#define FUNC_NIST_H

#ifdef __cplusplus
extern "C" {
#endif

void SetNistDebug(int enabled);

#define MAX(x, y) ((x) < (y) ? (y) : (x))
#define MIN(x, y) ((x) > (y) ? (y) : (x))
#define isPositive(x) ((x) > 0.e0 ? 1 : 0)
#define isNonPositive(x) ((x) <= 0.e0 ? 1 : 0)
#define isNegative(x) ((x) < 0.e0 ? 1 : 0)
#define isGreaterThanOne(x) ((x) > 1.e0 ? 1 : 0)
#define isZero(x) ((x) == 0.e0 ? 1 : 0)
#define isOne(x) ((x) == 1.e0 ? 1 : 0)

#define ALPHA 0.01			  // 显著性水平 >= 0.01
#define MAXNUMOFTEMPLATES 148 /* APERIODIC TEMPLATES: 148=>temp_length=9 */

/** NIST 测试函数声明 **/
/*
 * 错误码协议: 返回 0=成功; 1=参数非法; 2=模板数据不可用/解析失败; 3=内存分配失败。
 * 失败时 *out_pvalues 置 NULL 且已分配内存被释放;
 * J 不满足约束时 (随机游程类) 不视为错误, 返回全 0 的 pValues (与 NIST STS 语义一致)。
 */
int NonOverlappingTemplateMatchings_NIST(const unsigned char* epsilon, int n, int m,
                                         const unsigned char* tmpl_data, int tmpl_len,
                                         double** out_pvalues, int* out_pvalues_num);
int OverlappingTemplateMatchings_NIST(const unsigned char* epsilon, int n, int m, double* out_pvalue);
int RandomExcursions_NIST(const unsigned char* epsilon, int n, double** out_pvalues, int* out_pvalues_num);
int RandomExcursionsVariant_NIST(const unsigned char* epsilon, int n, double** out_pvalues, int* out_pvalues_num);

#ifdef __cplusplus
}
#endif

#endif // FUNC_NIST_H
