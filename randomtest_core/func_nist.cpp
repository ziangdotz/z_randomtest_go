#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>

#include "func_nist.h"
#include "cephes.h"

/** NIST 测试函数实现  **/

static int g_nist_debug_enabled = 0;

void SetNistDebug(int enabled)
{
	g_nist_debug_enabled = enabled ? 1 : 0;
}

#define DBG_PRINTF(...)                \
	do                                  \
	{                                   \
		if (g_nist_debug_enabled)         \
		{                                 \
			printf(__VA_ARGS__);            \
		}                                 \
	} while (0)

/* ---- 模板内存解析工具: 模板数据由 Go 侧经 go:embed 提供, 不再依赖文件系统 ---- */

static int tmpl_is_space(unsigned char c)
{
	return (c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == '\v');
}

/* 从内存缓冲中读取下一个整数; 返回 1=成功, 0=到达末尾, -1=解析错误 */
static int tmpl_next_int(const unsigned char **pp, const unsigned char *end, int *out)
{
	const unsigned char *p = *pp;
	long v;
	int sign = 1;

	while (p < end && tmpl_is_space(*p))
	{
		p++;
	}
	if (p >= end)
	{
		*pp = p;
		return 0;
	}
	if (*p == '-' || *p == '+')
	{
		if (*p == '-')
		{
			sign = -1;
		}
		p++;
	}
	if (p >= end || *p < '0' || *p > '9')
	{
		*pp = p;
		return -1;
	}
	v = 0;
	while (p < end && *p >= '0' && *p <= '9')
	{
		v = v * 10 + (*p - '0');
		p++;
	}
	*out = (int)(sign * v);
	*pp = p;
	return 1;
}

int NonOverlappingTemplateMatchings_NIST(const unsigned char *epsilon, int n, int m,
                                         const unsigned char *tmpl_data, int tmpl_len,
                                         double **out_pvalues, int *out_pvalues_num)
{
	const int N = 8;
	int numOfTemplates[100] = {0, 0, 2, 4, 6, 12, 20, 40, 74, 148, 284, 568, 1116, 2232, 4424, 8848, 17622, 35244, 70340, 140680, 281076, 562152};
	unsigned int W_obs, nu[6];
	unsigned int Wj[N];
	double *pValues = NULL;
	double sum, chi2, pValue, lambda, pi[6], varWj;
	int i, j, jj, k, match, SKIP, M, K = 5;
	unsigned char sequence[16]; /* must big than m */
	int bitInt, rc;
	const unsigned char *cursor, *end;

	*out_pvalues = NULL;
	*out_pvalues_num = 0;

	M = n / N;
	lambda = (M - m + 1) / pow(2, m);
	varWj = M * (1.0 / pow(2.0, m) - (2.0 * m - 1.0) / pow(2.0, 2.0 * m));
	if (((isNegative(lambda)) || (isZero(lambda))) ||
		(m > (int)(sizeof(sequence) / sizeof(unsigned char))) ||
		(tmpl_data == NULL) || (tmpl_len <= 0))
	{
		DBG_PRINTF("invalid parameters: lambda=%f, m=%d\n", lambda, m);
		return 1; /* 参数非法 */
	}

	if (numOfTemplates[m] < MAXNUMOFTEMPLATES)
	{
		SKIP = 1;
	}
	else
	{
		SKIP = (int)(numOfTemplates[m] / MAXNUMOFTEMPLATES);
	}
	numOfTemplates[m] = (int)numOfTemplates[m] / SKIP;
	sum = 0.0;
	for (i = 0; i < 2; i++)
	{ /* Compute Probabilities */
		pi[i] = exp(-lambda + i * log(lambda) - cephes_lgam(i + 1));
		sum += pi[i];
	}
	pi[0] = sum;
	for (i = 2; i <= K; i++)
	{ /* Compute Probabilities */
		pi[i - 1] = exp(-lambda + i * log(lambda) - cephes_lgam(i + 1));
		sum += pi[i - 1];
	}
	pi[K] = 1 - sum;

	if ((pValues = (double *)malloc(MAXNUMOFTEMPLATES * sizeof(double))) == NULL)
	{
		DBG_PRINTF("NonOverlappingTemplateMatchings: Insufficient memory.\n");
		return 3; /* 内存分配失败 */
	}
	*out_pvalues = pValues;
	*out_pvalues_num = MIN(MAXNUMOFTEMPLATES, numOfTemplates[m]);

	cursor = tmpl_data;
	end = tmpl_data + tmpl_len;
	for (jj = 0; jj < *out_pvalues_num; jj++)
	{
		sum = 0;
		for (k = 0; k < m; k++)
		{
			rc = tmpl_next_int(&cursor, end, &bitInt);
			if (rc != 1)
			{
				DBG_PRINTF("template data exhausted/invalid at template %d\n", jj);
				free(pValues);
				*out_pvalues = NULL;
				*out_pvalues_num = 0;
				return 2; /* 模板数据不可用 */
			}
			sequence[k] = (unsigned char)bitInt;
		}
		for (k = 0; k <= K; k++)
		{
			nu[k] = 0;
		}
		for (i = 0; i < N; i++)
		{
			W_obs = 0;
			for (j = 0; j < (M - m + 1); j++)
			{
				match = 1;
				for (k = 0; k < m; k++)
				{
					if ((int)sequence[k] != (int)epsilon[i * M + j + k])
					{
						match = 0;
						break;
					}
				}
				if (match == 1)
				{
					W_obs++;
					j += m - 1;
				}
			}
			Wj[i] = W_obs;
		}
		sum = 0;
		chi2 = 0.0; /* Compute Chi Square */
		for (i = 0; i < N; i++)
		{
			chi2 += pow(((double)Wj[i] - lambda) / pow(varWj, 0.5), 2);
		}
		pValue = cephes_igamc(N / 2.0, chi2 / 2.0);
		if (isNegative(pValue) || isGreaterThanOne(pValue))
		{
			DBG_PRINTF("WARNING: PVALue IS OUT OF RANGE.\n");
		}
		// printf("%9.6f %f %s %3d\n", chi2, pValue, pValue < ALPHA ? "FAILURE" : "SUCCESS", jj);
		pValues[jj] = pValue;
		if (SKIP > 1)
		{
			/* 跳过模板文件中后续 (SKIP - 1) 个模板 (每个模板 m 个整数) */
			for (i = 0; i < (SKIP - 1) * m; i++)
			{
				rc = tmpl_next_int(&cursor, end, &bitInt);
				if (rc != 1)
				{
					DBG_PRINTF("template data exhausted while skipping at template %d\n", jj);
					free(pValues);
					*out_pvalues = NULL;
					*out_pvalues_num = 0;
					return 2; /* 模板数据不可用 */
				}
			}
		}
	}
	return 0;
}

double Pr(int u, double eta)
{
	int l;
	double sum, p;

	if (u == 0)
	{
		p = exp(-eta);
	}
	else
	{
		sum = 0.0;
		for (l = 1; l <= u; l++)
		{
			sum += exp(-eta - u * log(2) + l * log(eta) - cephes_lgam(l + 1) + cephes_lgam(u) - cephes_lgam(l) - cephes_lgam(u - l + 1));
		}
		p = sum;
	}
	return p;
}

int OverlappingTemplateMatchings_NIST(const unsigned char *epsilon, int n, int m, double *out_pvalue)
{
	int i, k, match;
	double W_obs, eta, sum, chi2, pValue, lambda;
	int M, N, j, K = 5;
	unsigned int nu[6] = {0, 0, 0, 0, 0, 0};
	// double pi[6] = {0.143783, 0.139430, 0.137319, 0.124314, 0.106209, 0.348945};
	double pi[6] = {0.364091, 0.185659, 0.139381, 0.100571, 0.0704323, 0.139865};
	unsigned char *sequence;

	if (out_pvalue == NULL || epsilon == NULL || n <= 0 || m <= 0)
	{
		DBG_PRINTF("OVERLAPPING TEMPLATE: invalid parameters (n=%d, m=%d)\n", n, m);
		return 1; /* 参数非法 */
	}
	M = 1032;
	N = n / M;
	if (N <= 0)
	{
		DBG_PRINTF("OVERLAPPING TEMPLATE: n (%d) must be >= M (%d)\n", n, M);
		return 1; /* 参数非法 */
	}
	if ((sequence = (unsigned char *)calloc(m, sizeof(unsigned char))) == NULL)
	{
		DBG_PRINTF("OVERLAPPING TEMPLATE OF ALL ONES TEST\n");
		DBG_PRINTF("-------------------------------------\n");
		DBG_PRINTF("TEMPLATE DEFINITION: Insufficient memory, Overlapping Template Matchings test aborted!\n");
		return 3; /* 内存分配失败 */
	}
	for (i = 0; i < m; i++)
	{
		sequence[i] = 1;
	}
	lambda = (double)(M - m + 1) / pow(2, m);
	eta = lambda / 2.0;
	sum = 0.0;
	for (i = 0; i < K; i++)
	{ /* Compute Probabilities */
		pi[i] = Pr(i, eta);
		sum += pi[i];
	}
	pi[K] = 1 - sum;
	for (i = 0; i < N; i++)
	{
		W_obs = 0;
		for (j = 0; j < (M - m + 1); j++)
		{
			match = 1;
			for (k = 0; k < m; k++)
			{
				if (sequence[k] != epsilon[i * M + j + k])
				{
					match = 0;
				}
			}
			if (match == 1)
			{
				W_obs++;
			}
		}
		if (W_obs <= 4)
		{
			nu[(int)W_obs]++;
		}
		else
		{
			nu[K]++;
		}
	}
	sum = 0;
	chi2 = 0.0; /* Compute Chi Square */
	for (i = 0; i < (K + 1); i++)
	{
		chi2 += pow((double)nu[i] - (double)N * pi[i], 2) / ((double)N * pi[i]);
		sum += nu[i];
	}
	pValue = cephes_igamc(K / 2.0, chi2 / 2.0);

	free(sequence);

	if (isNegative(pValue) || isGreaterThanOne(pValue))
	{
		DBG_PRINTF("WARNING: PVALue IS OUT OF RANGE.\n");
	}
	// printf("%f %s\n\n", pValue, pValue < ALPHA ? "FAILURE" : "SUCCESS");
	*out_pvalue = pValue;
	return 0;
}

int RandomExcursions_NIST(const unsigned char *epsilon, int n, double **out_pvalues, int *out_pvalues_num)
{
	double *pValues = NULL;

	double pValue;
	int J, i, k, b, j, x;
	int *S_k = NULL, *cycle = NULL, cycleStart, cycleStop;
	double constraint, nu[6][8], sum;
	int stateX[8] = {-4, -3, -2, -1, 1, 2, 3, 4};
	int counter[8] = {0, 0, 0, 0, 0, 0, 0, 0};
	double pi[5][6] = {{0.0000000000, 0.00000000000, 0.00000000000, 0.00000000000, 0.00000000000, 0.0000000000},
					   {0.5000000000, 0.25000000000, 0.12500000000, 0.06250000000, 0.03125000000, 0.0312500000},
					   {0.7500000000, 0.06250000000, 0.04687500000, 0.03515625000, 0.02636718750, 0.0791015625},
					   {0.8333333333, 0.02777777778, 0.02314814815, 0.01929012346, 0.01607510288, 0.0803755143},
					   {0.8750000000, 0.01562500000, 0.01367187500, 0.01196289063, 0.01046752930, 0.0732727051}};

	*out_pvalues = NULL;
	*out_pvalues_num = 0;
	if (epsilon == NULL || n <= 0)
	{
		DBG_PRINTF("Random Excursions Test: invalid parameters.\n");
		return 1; /* 参数非法 */
	}
	if ((pValues = (double *)malloc(8 * sizeof(double))) == NULL)
	{
		DBG_PRINTF("Random Excursions Test: Insufficient memory for pValues.\n");
		return 3; /* 内存分配失败 */
	}
	*out_pvalues = pValues;
	*out_pvalues_num = 8;
	if (((S_k = (int *)calloc(n, sizeof(int))) == NULL) ||
		((cycle = (int *)calloc(MAX(1000, n / 100), sizeof(int))) == NULL))
	{
		DBG_PRINTF("Random Excursions Test: Insufficent Work Space Allocated.\n");
		if (S_k != NULL)
		{
			free(S_k);
		}
		if (cycle != NULL)
		{
			free(cycle);
		}
		free(pValues);
		*out_pvalues = NULL;
		*out_pvalues_num = 0;
		return 3; /* 内存分配失败 */
	}
	J = 0; /* DETERMINE CYCLES */
	S_k[0] = 2 * (int)epsilon[0] - 1;
	for (i = 1; i < n; i++)
	{
		S_k[i] = S_k[i - 1] + 2 * epsilon[i] - 1;
		if (S_k[i] == 0)
		{
			J++;
			if (J > MAX(1000, n / 100))
			{
				DBG_PRINTF("ERROR IN FUNCTION randomExcursions: EXCEEDING THE MAX NUMBER OF CYCLES EXPECTED\n.");
				free(S_k);
				free(cycle);
				free(pValues);
				*out_pvalues = NULL;
				*out_pvalues_num = 0;
				return 1; /* 参数/数据越界 */
			}
			cycle[J] = i;
		}
	}
	if (S_k[n - 1] != 0)
	{
		J++;
	}
	cycle[J] = n;
	constraint = MAX(0.005 * pow(n, 0.5), 500);
	if (J < constraint)
	{
		for (i = 0; i < 8; i++)
		{
			pValues[i] = 0.0; /* 测试不适用, 与 NIST STS 语义一致 (非错误) */
		}
	}
	else
	{
		cycleStart = 0;
		cycleStop = cycle[1];
		for (k = 0; k < 6; k++)
		{
			for (i = 0; i < 8; i++)
			{
				nu[k][i] = 0.;
			}
		}
		for (j = 1; j <= J; j++)
		{ /* FOR EACH CYCLE */
			for (i = 0; i < 8; i++)
			{
				counter[i] = 0;
			}
			for (i = cycleStart; i < cycleStop; i++)
			{
				if ((S_k[i] >= 1 && S_k[i] <= 4) ||
					(S_k[i] >= -4 && S_k[i] <= -1))
				{
					if (S_k[i] < 0)
					{
						b = 4;
					}
					else
					{
						b = 3;
					}
					counter[S_k[i] + b]++;
				}
			}
			cycleStart = cycle[j] + 1;
			if (j < J)
			{
				cycleStop = cycle[j + 1];
			}

			for (i = 0; i < 8; i++)
			{
				if ((counter[i] >= 0) && (counter[i] <= 4))
				{
					nu[counter[i]][i]++;
				}
				else if (counter[i] >= 5)
				{
					nu[5][i]++;
				}
			}
		}
		for (i = 0; i < 8; i++)
		{
			x = stateX[i];
			sum = 0.;
			for (k = 0; k < 6; k++)
			{
				sum += pow(nu[k][i] - J * pi[(int)fabs(x)][k], 2) / (J * pi[(int)fabs(x)][k]);
			}
			pValue = cephes_igamc(2.5, sum / 2.0);
			if (isNegative(pValue) || isGreaterThanOne(pValue))
			{
				DBG_PRINTF("WARNING: PVALue IS OUT OF RANGE.\n");
			}
			// printf("%s\t\tx = %2d chi^2 = %9.6f pValue = %f\n", pValue < ALPHA ? "FAILURE" : "SUCCESS", x, sum, pValue);
			pValues[i] = pValue;
		}
	}
	free(S_k);
	free(cycle);
	return 0;
}

int RandomExcursionsVariant_NIST(const unsigned char *epsilon, int n, double **out_pvalues, int *out_pvalues_num)
{
	double *pValues = NULL;

	double pValue;
	int *S_k, J, i, constraint, p, x, count;
	int stateX[18] = {-9, -8, -7, -6, -5, -4, -3, -2, -1, 1, 2, 3, 4, 5, 6, 7, 8, 9};

	*out_pvalues = NULL;
	*out_pvalues_num = 0;
	if (epsilon == NULL || n <= 0)
	{
		DBG_PRINTF("RANDOM EXCURSIONS VARIANT: invalid parameters.\n");
		return 1; /* 参数非法 */
	}
	if ((pValues = (double *)malloc(18 * sizeof(double))) == NULL)
	{
		DBG_PRINTF("RANDOM EXCURSIONS VARIANT: Insufficient memory for pValues.\n");
		return 3; /* 内存分配失败 */
	}
	*out_pvalues = pValues;
	*out_pvalues_num = 18;
	if ((S_k = (int *)calloc(n, sizeof(int))) == NULL)
	{
		DBG_PRINTF("RANDOM EXCURSIONS VARIANT: Insufficent memory allocated.\n");
		free(pValues);
		*out_pvalues = NULL;
		*out_pvalues_num = 0;
		return 3; /* 内存分配失败 */
	}
	J = 0;
	S_k[0] = 2 * (int)epsilon[0] - 1;
	for (i = 1; i < n; i++)
	{
		S_k[i] = S_k[i - 1] + 2 * epsilon[i] - 1;
		if (S_k[i] == 0)
		{
			J++;
		}
	}
	if (S_k[n - 1] != 0)
	{
		J++;
	}
	constraint = (int)MAX(0.005 * pow(n, 0.5), 500);
	if (J < constraint)
	{
		DBG_PRINTF("WARNING: TEST NOT APPLICABLE. THERE ARE AN INSUFFICIENT NUMBER OF CYCLES.\n");
		DBG_PRINTF("--------------------------\n");
		for (i = 0; i < 18; i++)
		{
			pValues[i] = 0.0; /* 测试不适用, 与 NIST STS 语义一致 (非错误) */
		}
	}
	else
	{
		for (p = 0; p <= 17; p++)
		{
			x = stateX[p];
			count = 0;
			for (i = 0; i < n; i++)
			{
				if (S_k[i] == x)
				{
					count++;
				}
			}
			pValue = erfc(fabs(count - J) / (sqrt(2.0 * J * (4.0 * fabs(x) - 2))));
			if (isNegative(pValue) || isGreaterThanOne(pValue))
			{
				DBG_PRINTF("WARNING: PVALue IS OUT OF RANGE.\n");
			}
			// printf("%s\t(x = %2d) Total visits = %4d; p-value = %f\n", pValue < ALPHA ? "FAILURE" : "SUCCESS", x, count, pValue);
			pValues[p] = pValue;
		}
	}
	free(S_k);
	return 0;
}
