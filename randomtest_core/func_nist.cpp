#include <stdio.h>
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

double *NonOverlappingTemplateMatchings_NIST(unsigned char *epsilon, int n, int m, char *template_filepath, int *pValues_num)
{
	double *pValues = (double *)malloc(MAXNUMOFTEMPLATES * sizeof(double));
	*pValues_num = MAXNUMOFTEMPLATES;

	// NOTE:  Should additional templates lengths beyond 21 be desired, they must first be constructed, saved into files and then the corresponding number of nonperiodic templates for that file be stored in the m-th position in the numOfTemplates variable.
	const int N = 8;
	int numOfTemplates[100] = {0, 0, 2, 4, 6, 12, 20, 40, 74, 148, 284, 568, 1116, 2232, 4424, 8848, 17622, 35244, 70340, 140680, 281076, 562152};
	unsigned int bit, W_obs, nu[6];
	unsigned int Wj[N];
	FILE *fp = NULL;
	double sum, chi2, pValue, lambda, pi[6], varWj;
	int i, j, jj, k, match, SKIP, M, K = 5;
	unsigned char sequence[16]; // must big than m
	M = n / N;
	lambda = (M - m + 1) / pow(2, m);
	varWj = M * (1.0 / pow(2.0, m) - (2.0 * m - 1.0) / pow(2.0, 2.0 * m));
	if (((isNegative(lambda)) || (isZero(lambda))) ||
		((fp = fopen(template_filepath, "r")) == NULL) || (m > sizeof(sequence) / sizeof(unsigned char)))
	{
		DBG_PRINTF("invalid parameters: lambda=%f, m=%d", lambda, m);
	}
	else
	{
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
		*pValues_num = MIN(MAXNUMOFTEMPLATES, numOfTemplates[m]);
		for (jj = 0; jj < *pValues_num; jj++)
		{
			sum = 0;
			for (k = 0; k < m; k++)
			{
				fscanf(fp, "%d", &bit);
				sequence[k] = bit;
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
				fseek(fp, (long)(SKIP - 1) * 2 * m, SEEK_CUR);
			}
		}
	}
	if (fp != NULL)
	{
		fclose(fp);
	}
	return pValues;
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
double OverlappingTemplateMatchings_NIST(unsigned char *epsilon, int n, int m)
{
	int i, k, match;
	double W_obs, eta, sum, chi2, pValue, lambda;
	int M, N, j, K = 5;
	unsigned int nu[6] = {0, 0, 0, 0, 0, 0};
	// double pi[6] = {0.143783, 0.139430, 0.137319, 0.124314, 0.106209, 0.348945};
	double pi[6] = {0.364091, 0.185659, 0.139381, 0.100571, 0.0704323, 0.139865};
	unsigned char *sequence;
	M = 1032;
	N = n / M;
	if ((sequence = (unsigned char *)calloc(m, sizeof(unsigned char))) == NULL)
	{
		DBG_PRINTF("OVERLAPPING TEMPLATE OF ALL ONES TEST\n");
		DBG_PRINTF("-------------------------------------\n");
		DBG_PRINTF("TEMPLATE DEFINITION: Insufficient memory, Overlapping Template Matchings test aborted!\n");
	}
	else
	{
		for (i = 0; i < m; i++)
		{
			sequence[i] = 1;
		}
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
	return pValue;
}

double *RandomExcursions_NIST(unsigned char *epsilon, int n, int *pValues_num)
{
	double *pValues = (double *)malloc(8 * sizeof(double));
	*pValues_num = 8;

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
		return NULL;
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
				return NULL;
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
			pValues[i] = 0.0;
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
	return pValues;
}

double *RandomExcursionsVariant_NIST(unsigned char *epsilon, int n, int *pValues_num)
{
	double *pValues = (double *)malloc(18 * sizeof(double));
	*pValues_num = 18;

	double pValue;
	int *S_k, J, i, constraint, p, x, count;
	int stateX[18] = {-9, -8, -7, -6, -5, -4, -3, -2, -1, 1, 2, 3, 4, 5, 6, 7, 8, 9};
	if ((S_k = (int *)calloc(n, sizeof(int))) == NULL)
	{
		DBG_PRINTF("RANDOM EXCURSIONS VARIANT: Insufficent memory allocated.\n");
		return NULL;
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
			pValues[i] = 0.0;
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
	return pValues;
}
