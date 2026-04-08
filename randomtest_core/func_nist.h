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

double* NonOverlappingTemplateMatchings_NIST(unsigned char* epsilon, int n, int m, char* template_filepath, int* pvalues_num);
double OverlappingTemplateMatchings_NIST(unsigned char* epsilon, int n, int m);
double* RandomExcursions_NIST(unsigned char* epsilon, int n, int* pvalues_num);
double* RandomExcursionsVariant_NIST(unsigned char* epsilon, int n, int* pvalues_num);

#ifdef __cplusplus
}
#endif

#endif // FUNC_NIST_H
